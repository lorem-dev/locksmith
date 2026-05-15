package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lorem-dev/locksmith/internal/log"
)

// SSETransport implements the legacy SSE MCP transport.
// It opens GET <baseURL>/sse for server messages and POSTs to the endpoint
// URL received in the initial "endpoint" SSE event.
type SSETransport struct {
	baseURL  string
	client   *http.Client
	endpoint string
	msgCh    chan []byte
	cancel   context.CancelFunc

	staticHeaders http.Header
	auth          *authState
}

func (t *SSETransport) Connect(ctx context.Context) (<-chan []byte, error) {
	t.msgCh = make(chan []byte, msgChanBuf)

	sseURL := strings.TrimRight(t.baseURL, "/") + "/sse"
	//nolint:bodyclose // body is closed by the stream goroutine or explicitly on retry
	resp, err := t.openSSEStream(ctx, sseURL)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		resp.Body.Close() //nolint:errcheck
		ok, authErr := t.ensureAuth(ctx)
		if authErr != nil {
			return nil, authErr
		}
		if !ok {
			return nil, &httpStatusError{StatusCode: resp.StatusCode, URL: sseURL}
		}
		log.Debug().Int("status", resp.StatusCode).Msg("SSE: 401/403 on GET, reopening with auth")
		resp, err = t.openSSEStream(ctx, sseURL) //nolint:bodyclose // body is closed by the goroutine started below
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close() //nolint:errcheck
			return nil, &httpStatusError{StatusCode: resp.StatusCode, URL: sseURL}
		}
	}

	endpointCh := make(chan string, 1)
	streamCtx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	go func() {
		defer resp.Body.Close() //nolint:errcheck // close in stream goroutine; error not actionable
		defer close(t.msgCh)
		endpointSent := false
		for event := range parseSSE(resp.Body) {
			if !endpointSent && event.Type == "endpoint" {
				endpointCh <- event.Data
				endpointSent = true
				continue
			}
			select {
			case t.msgCh <- []byte(event.Data):
			case <-streamCtx.Done():
				return
			}
		}
	}()

	select {
	case ep := <-endpointCh:
		base, err := url.Parse(t.baseURL)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("parsing base URL: %w", err)
		}
		epURL, err := url.Parse(ep)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("parsing endpoint URL %q: %w", ep, err)
		}
		t.endpoint = base.ResolveReference(epURL).String()
		log.Debug().Str("endpoint", RedactURL(t.endpoint)).Msg("SSE: endpoint resolved")
		return t.msgCh, nil
	case <-time.After(sseEndpointWait):
		cancel()
		log.Debug().Str("url", RedactURL(sseURL)).Msg("SSE: timeout waiting for endpoint event")
		return nil, fmt.Errorf("timeout waiting for endpoint event from %s", RedactURL(sseURL))
	case <-ctx.Done():
		cancel()
		log.Debug().Msg("SSE: context canceled before endpoint received")
		return nil, fmt.Errorf("context canceled before endpoint received: %w", ctx.Err())
	}
}

func (t *SSETransport) Send(ctx context.Context, msg []byte) error {
	if t.endpoint == "" {
		return fmt.Errorf("SSE transport not connected: call Connect first")
	}
	resp, err := t.postEndpointOnce(ctx, msg)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	log.Debug().Str("endpoint", RedactURL(t.endpoint)).Int("status", resp.StatusCode).Msg("SSE: POST response")

	if t.shouldRetryWithAuth(resp.StatusCode) {
		resp.Body.Close() //nolint:errcheck
		ok, authErr := t.ensureAuth(ctx)
		if authErr != nil {
			return authErr
		}
		if !ok {
			return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, RedactURL(t.endpoint))
		}
		log.Debug().Int("status", resp.StatusCode).Msg("SSE: 401/403 on POST, retrying with auth")
		resp, err = t.postEndpointOnce(ctx, msg)
		if err != nil {
			return fmt.Errorf("retrying after auth: %w", err)
		}
		defer resp.Body.Close() //nolint:errcheck
		log.Debug().Int("status", resp.StatusCode).Msg("SSE: retry response")
	}
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, RedactURL(t.endpoint))
	}
	return nil
}

// postEndpointOnce builds and dispatches one POST to t.endpoint with
// the transport's effective headers. It does NOT retry. The caller is
// responsible for closing resp.Body.
func (t *SSETransport) postEndpointOnce(ctx context.Context, msg []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytesReader(msg))
	if err != nil {
		return nil, fmt.Errorf("building POST request: %w", err)
	}
	req.ContentLength = int64(len(msg))
	req.Header.Set("Content-Type", "application/json")
	for k, vs := range t.effectiveHeaders() {
		req.Header[k] = vs
	}
	log.Debug().Str("endpoint", RedactURL(t.endpoint)).Int("len", len(msg)).Msg("SSE: POST")
	resp, err := t.client.Do(req)
	if err != nil {
		log.Debug().Err(err).Str("endpoint", RedactURL(t.endpoint)).Msg("SSE: POST failed")
		return nil, fmt.Errorf("sending message: %w", err)
	}
	return resp, nil
}

// shouldRetryWithAuth reports whether the response status warrants an
// auth resolve and retry. Mirrors StreamableHTTP.shouldRetryWithAuth.
func (t *SSETransport) shouldRetryWithAuth(status int) bool {
	if t.auth == nil {
		return false
	}
	if status != http.StatusUnauthorized && status != http.StatusForbidden {
		return false
	}
	return !t.auth.Attempted()
}

func (t *SSETransport) Close() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// openSSEStream issues GET <baseURL>/sse with the transport's effective
// headers and returns the response on success. The caller takes
// ownership of resp.Body (must close it). 4xx responses are returned to
// the caller for retry / error decisions; they do NOT auto-close.
func (t *SSETransport) openSSEStream(ctx context.Context, sseURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for k, vs := range t.effectiveHeaders() {
		req.Header[k] = vs
	}
	log.Debug().Str("url", RedactURL(sseURL)).Msg("SSE: GET")
	resp, err := t.client.Do(req)
	if err != nil {
		log.Debug().Err(err).Str("url", RedactURL(sseURL)).Msg("SSE: GET failed")
		return nil, fmt.Errorf("connecting to %s: %w", RedactURL(sseURL), err)
	}
	log.Debug().Str("url", RedactURL(sseURL)).Int("status", resp.StatusCode).Msg("SSE: GET response")
	return resp, nil
}

func (t *SSETransport) effectiveHeaders() http.Header {
	out := make(http.Header, len(t.staticHeaders))
	for k, vs := range t.staticHeaders {
		out[k] = append([]string(nil), vs...)
	}
	if t.auth == nil {
		return out
	}
	for k, vs := range t.auth.Headers() {
		out[k] = append([]string(nil), vs...)
	}
	return out
}

// ensureAuth resolves auth via the shared authState. Returns (true, nil)
// when headers were resolved on this or a prior call, (false, nil) if
// auth is unavailable, or (false, err) if the resolve failed.
func (t *SSETransport) ensureAuth(ctx context.Context) (bool, error) {
	if t.auth == nil {
		return false, nil
	}
	if err := t.auth.resolveOnce(ctx); err != nil {
		return false, fmt.Errorf("resolving auth headers: %w", err)
	}
	return t.auth.Headers() != nil, nil
}

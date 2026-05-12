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
	headers  http.Header
	client   *http.Client
	endpoint string
	msgCh    chan []byte
	cancel   context.CancelFunc
}

func (t *SSETransport) Connect(ctx context.Context) (<-chan []byte, error) {
	t.msgCh = make(chan []byte, msgChanBuf)

	sseURL := strings.TrimRight(t.baseURL, "/") + "/sse"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	for k, vs := range t.headers {
		req.Header[k] = vs
	}

	log.Debug().Str("url", RedactURL(sseURL)).Msg("SSE: GET")
	resp, err := t.client.Do(req) //nolint:bodyclose // body is closed by the goroutine started below
	if err != nil {
		log.Debug().Err(err).Str("url", RedactURL(sseURL)).Msg("SSE: GET failed")
		return nil, fmt.Errorf("connecting to %s: %w", RedactURL(sseURL), err)
	}
	log.Debug().Str("url", RedactURL(sseURL)).Int("status", resp.StatusCode).Msg("SSE: GET response")

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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, bytesReader(msg))
	if err != nil {
		return fmt.Errorf("building POST request: %w", err)
	}
	req.ContentLength = int64(len(msg))
	req.Header.Set("Content-Type", "application/json")
	for k, vs := range t.headers {
		req.Header[k] = vs
	}
	log.Debug().Str("endpoint", RedactURL(t.endpoint)).Int("len", len(msg)).Msg("SSE: POST")
	resp, err := t.client.Do(req)
	if err != nil {
		log.Debug().Err(err).Str("endpoint", RedactURL(t.endpoint)).Msg("SSE: POST failed")
		return fmt.Errorf("sending message: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	log.Debug().Str("endpoint", RedactURL(t.endpoint)).Int("status", resp.StatusCode).Msg("SSE: POST response")
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from %s", resp.StatusCode, RedactURL(t.endpoint))
	}
	return nil
}

func (t *SSETransport) Close() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

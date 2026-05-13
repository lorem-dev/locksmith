package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/lorem-dev/locksmith/internal/log"
)

// StreamableHTTP implements the MCP Streamable HTTP transport (spec 2025-03-26).
// Each client message is sent as a POST; the response may be JSON or SSE.
type StreamableHTTP struct {
	baseURL string
	client  *http.Client
	msgCh   chan []byte
	cancel  context.CancelFunc

	staticHeaders http.Header
	resolveAuth   HeaderResolver

	authMu     sync.Mutex
	cachedAuth http.Header
}

func (t *StreamableHTTP) Connect(ctx context.Context) (<-chan []byte, error) {
	t.msgCh = make(chan []byte, msgChanBuf)
	getCtx, cancel := context.WithCancel(ctx)
	t.cancel = cancel
	go t.runGetStream(getCtx)
	return t.msgCh, nil
}

func (t *StreamableHTTP) runGetStream(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.baseURL, nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "text/event-stream")
	for k, vs := range t.effectiveHeaders() {
		req.Header[k] = vs
	}
	log.Debug().Str("url", RedactURL(t.baseURL)).Msg("HTTP: GET (server notifications stream)")
	resp, err := t.client.Do(req)
	if err != nil {
		log.Debug().Err(err).Str("url", RedactURL(t.baseURL)).Msg("HTTP: GET stream failed")
		return
	}
	log.Debug().Str("url", RedactURL(t.baseURL)).Int("status", resp.StatusCode).Msg("HTTP: GET stream response")
	if resp.StatusCode == http.StatusMethodNotAllowed ||
		resp.StatusCode == http.StatusNotFound ||
		resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusForbidden {
		log.Debug().
			Int("status", resp.StatusCode).
			Msg("HTTP: GET stream rejected; closing quietly, POST will handle auth")
		resp.Body.Close() //nolint:errcheck
		return
	}
	defer resp.Body.Close() //nolint:errcheck
	for event := range parseSSE(resp.Body) {
		select {
		case t.msgCh <- []byte(event.Data):
		case <-ctx.Done():
			return
		}
	}
}

func (t *StreamableHTTP) Send(ctx context.Context, msg []byte) error {
	resp, err := t.postOnce(ctx, msg)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck
	ct := resp.Header.Get("Content-Type")
	log.Debug().
		Str("url", RedactURL(t.baseURL)).
		Int("status", resp.StatusCode).
		Str("content_type", ct).
		Msg("HTTP: POST response")

	if t.shouldRetryWithAuth(resp.StatusCode) {
		resp.Body.Close() //nolint:errcheck // body of the failed request not needed
		ok, authErr := t.ensureAuth(ctx)
		if authErr != nil {
			return authErr
		}
		if !ok {
			return &httpStatusError{StatusCode: resp.StatusCode, URL: t.baseURL}
		}
		log.Debug().Int("status", resp.StatusCode).Msg("HTTP: 401/403 received, retrying with auth")
		resp, err = t.postOnce(ctx, msg)
		if err != nil {
			return fmt.Errorf("retrying after auth: %w", err)
		}
		defer resp.Body.Close() //nolint:errcheck
		ct = resp.Header.Get("Content-Type")
		log.Debug().Int("status", resp.StatusCode).Str("content_type", ct).Msg("HTTP: retry response")
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return &httpStatusError{StatusCode: resp.StatusCode, URL: t.baseURL}
	}
	if strings.HasPrefix(ct, "text/event-stream") {
		for event := range parseSSE(resp.Body) {
			select {
			case t.msgCh <- []byte(event.Data):
			case <-ctx.Done():
				return fmt.Errorf("forwarding SSE response: %w", ctx.Err())
			}
		}
	} else {
		body, err := readBody(resp)
		if err != nil {
			return err
		}
		if len(body) > 0 {
			select {
			case t.msgCh <- body:
			case <-ctx.Done():
				return fmt.Errorf("forwarding JSON response: %w", ctx.Err())
			}
		}
	}
	return nil
}

// postOnce builds and dispatches one POST with the transport's effective
// headers. It does NOT retry. The caller is responsible for closing
// resp.Body.
func (t *StreamableHTTP) postOnce(ctx context.Context, msg []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL, bytesReader(msg))
	if err != nil {
		return nil, fmt.Errorf("building POST request: %w", err)
	}
	req.ContentLength = int64(len(msg))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, vs := range t.effectiveHeaders() {
		req.Header[k] = vs
	}
	log.Debug().Str("url", RedactURL(t.baseURL)).Int("len", len(msg)).Msg("HTTP: POST")
	resp, err := t.client.Do(req)
	if err != nil {
		log.Debug().Err(err).Str("url", RedactURL(t.baseURL)).Msg("HTTP: POST failed")
		return nil, fmt.Errorf("sending message: %w", err)
	}
	return resp, nil
}

// shouldRetryWithAuth reports whether the given HTTP status is an
// auth-related rejection that warrants resolving the auth header and
// retrying once. Only valid before ensureAuth has succeeded; afterwards
// 401/403 from the server is a genuine failure (the cached auth is
// wrong or expired) and is propagated.
func (t *StreamableHTTP) shouldRetryWithAuth(status int) bool {
	if status != http.StatusUnauthorized && status != http.StatusForbidden {
		return false
	}
	t.authMu.Lock()
	defer t.authMu.Unlock()
	return t.cachedAuth == nil
}

func (t *StreamableHTTP) Close() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// effectiveHeaders returns the headers to attach to the next request:
// staticHeaders alone before authentication, staticHeaders + cachedAuth
// after. The returned header is a fresh copy safe for the caller to
// mutate (e.g. set Content-Type) without affecting future calls.
func (t *StreamableHTTP) effectiveHeaders() http.Header {
	out := make(http.Header, len(t.staticHeaders))
	for k, vs := range t.staticHeaders {
		out[k] = append([]string(nil), vs...)
	}
	t.authMu.Lock()
	for k, vs := range t.cachedAuth {
		out[k] = append([]string(nil), vs...)
	}
	t.authMu.Unlock()
	return out
}

// ensureAuth resolves the auth headers if they have not been resolved
// yet. It returns true if auth is available (already cached or just
// resolved), false if no resolver is configured. A non-nil error means
// the resolver itself failed; the caller should propagate it.
func (t *StreamableHTTP) ensureAuth(ctx context.Context) (bool, error) {
	if t.resolveAuth == nil {
		return false, nil
	}
	t.authMu.Lock()
	defer t.authMu.Unlock()
	if t.cachedAuth != nil {
		return true, nil
	}
	h, err := t.resolveAuth(ctx)
	if err != nil {
		return false, fmt.Errorf("resolving auth headers: %w", err)
	}
	t.cachedAuth = h
	return true, nil
}

// AutoTransport tries Streamable HTTP first; falls back to SSE on 404/405.
// It owns a single output channel (outCh) and forwards messages from whichever
// underlying transport is active, so the caller always reads from the same channel.
type AutoTransport struct {
	baseURL string
	client  *http.Client

	staticHeaders http.Header
	resolveAuth   HeaderResolver

	inner  Transport
	outCh  chan []byte
	cancel context.CancelFunc
}

func (t *AutoTransport) Connect(ctx context.Context) (<-chan []byte, error) {
	t.outCh = make(chan []byte, msgChanBuf)
	fwdCtx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	log.Debug().Str("url", RedactURL(t.baseURL)).Msg("Auto: trying Streamable HTTP first")
	t.inner = &StreamableHTTP{
		baseURL:       t.baseURL,
		staticHeaders: t.staticHeaders,
		resolveAuth:   t.resolveAuth,
		client:        t.client,
	}
	innerCh, err := t.inner.Connect(fwdCtx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("connecting Streamable HTTP transport: %w", err)
	}
	go t.forwardFrom(fwdCtx, innerCh)
	return t.outCh, nil
}

func (t *AutoTransport) forwardFrom(ctx context.Context, ch <-chan []byte) {
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			select {
			case t.outCh <- msg:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (t *AutoTransport) Send(ctx context.Context, msg []byte) error {
	err := t.inner.Send(ctx, msg)
	if err == nil {
		return nil
	}
	var httpErr *httpStatusError
	if errors.As(err, &httpErr) && isFallbackStatus(httpErr.StatusCode) {
		log.Debug().Int("status", httpErr.StatusCode).Str("url", RedactURL(t.baseURL)).Msg("Auto: falling back to SSE")
		t.inner.Close() //nolint:errcheck // fallback path; close error not actionable
		sse := &SSETransport{
			baseURL:       t.baseURL,
			staticHeaders: t.staticHeaders,
			resolveAuth:   t.resolveAuth,
			client:        t.client,
		}
		sseCh, connectErr := sse.Connect(ctx)
		if connectErr != nil {
			return fmt.Errorf("falling back to SSE transport: %w", connectErr)
		}
		t.inner = sse
		go t.forwardFrom(ctx, sseCh)
		if sendErr := sse.Send(ctx, msg); sendErr != nil {
			return fmt.Errorf("sending via SSE fallback: %w", sendErr)
		}
		return nil
	}
	return fmt.Errorf("sending via Streamable HTTP: %w", err)
}

func isFallbackStatus(code int) bool {
	return code == http.StatusNotFound || code == http.StatusMethodNotAllowed
}

func (t *AutoTransport) Close() error {
	if t.cancel != nil {
		t.cancel()
	}
	if t.inner != nil {
		if err := t.inner.Close(); err != nil {
			return fmt.Errorf("closing inner transport: %w", err)
		}
	}
	return nil
}

// httpStatusError carries an HTTP status code for transport fallback decisions.
type httpStatusError struct {
	StatusCode int
	URL        string
}

func (e *httpStatusError) Error() string {
	return fmt.Sprintf("HTTP %d from %s", e.StatusCode, RedactURL(e.URL))
}

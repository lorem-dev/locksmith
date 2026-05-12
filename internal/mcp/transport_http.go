package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// StreamableHTTP implements the MCP Streamable HTTP transport (spec 2025-03-26).
// Each client message is sent as a POST; the response may be JSON or SSE.
type StreamableHTTP struct {
	baseURL string
	headers http.Header
	client  *http.Client
	msgCh   chan []byte
	cancel  context.CancelFunc
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
	for k, vs := range t.headers {
		req.Header[k] = vs
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return
	}
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.baseURL, bytesReader(msg))
	if err != nil {
		return fmt.Errorf("building POST request: %w", err)
	}
	req.ContentLength = int64(len(msg))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	for k, vs := range t.headers {
		req.Header[k] = vs
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("sending message: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode >= http.StatusBadRequest {
		return &httpStatusError{StatusCode: resp.StatusCode, URL: t.baseURL}
	}
	ct := resp.Header.Get("Content-Type")
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

func (t *StreamableHTTP) Close() error {
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// AutoTransport tries Streamable HTTP first; falls back to SSE on 404/405.
// It owns a single output channel (outCh) and forwards messages from whichever
// underlying transport is active, so the caller always reads from the same channel.
type AutoTransport struct {
	baseURL string
	headers http.Header
	client  *http.Client
	inner   Transport
	outCh   chan []byte
	cancel  context.CancelFunc
}

func (t *AutoTransport) Connect(ctx context.Context) (<-chan []byte, error) {
	t.outCh = make(chan []byte, msgChanBuf)
	fwdCtx, cancel := context.WithCancel(ctx)
	t.cancel = cancel

	t.inner = &StreamableHTTP{baseURL: t.baseURL, headers: t.headers, client: t.client}
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
		t.inner.Close() //nolint:errcheck // fallback path; close error not actionable
		sse := &SSETransport{baseURL: t.baseURL, headers: t.headers, client: t.client}
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
	return fmt.Sprintf("HTTP %d from %s", e.StatusCode, e.URL)
}

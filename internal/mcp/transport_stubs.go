package mcp

import (
	"context"
	"net/http"
)

// Stubs - replaced by transport_sse.go (Task 6) and transport_http.go (Task 7).

type SSETransport struct {
	baseURL  string
	headers  http.Header
	client   *http.Client
	endpoint string
	msgCh    chan []byte
	cancel   context.CancelFunc
}

func (t *SSETransport) Connect(_ context.Context) (<-chan []byte, error) { return nil, nil }
func (t *SSETransport) Send(_ context.Context, _ []byte) error           { return nil }
func (t *SSETransport) Close() error                                      { return nil }

type StreamableHTTP struct {
	baseURL string
	headers http.Header
	client  *http.Client
	msgCh   chan []byte
	cancel  context.CancelFunc
}

func (t *StreamableHTTP) Connect(_ context.Context) (<-chan []byte, error) { return nil, nil }
func (t *StreamableHTTP) Send(_ context.Context, _ []byte) error           { return nil }
func (t *StreamableHTTP) Close() error                                      { return nil }

type AutoTransport struct {
	baseURL string
	headers http.Header
	client  *http.Client
	inner   Transport
	outCh   chan []byte
	cancel  context.CancelFunc
}

func (t *AutoTransport) Connect(_ context.Context) (<-chan []byte, error) { return nil, nil }
func (t *AutoTransport) Send(_ context.Context, _ []byte) error           { return nil }
func (t *AutoTransport) Close() error                                      { return nil }

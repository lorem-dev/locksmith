package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	sseEventChanBuf  = 16
	msgChanBuf       = 64
	httpClientTimout = 30 * time.Second
	sseEndpointWait  = 10 * time.Second
)

// Transport abstracts the HTTP transport for a remote MCP server.
type Transport interface {
	// Connect establishes the connection and returns a channel of server messages.
	Connect(ctx context.Context) (<-chan []byte, error)
	// Send delivers a client message to the server.
	Send(ctx context.Context, msg []byte) error
	// Close terminates the transport.
	Close() error
}

// SSEEvent is a parsed server-sent event.
type SSEEvent struct {
	Type string
	Data string
}

// parseSSE reads SSE events from r, sending each complete event to the returned channel.
// The channel is closed when r is exhausted or returns an error.
func parseSSE(r io.Reader) <-chan SSEEvent {
	ch := make(chan SSEEvent, sseEventChanBuf)
	go func() {
		defer close(ch)
		scanner := bufio.NewScanner(r)
		var event SSEEvent
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event:"):
				event.Type = strings.TrimSpace(line[6:])
			case strings.HasPrefix(line, "data:"):
				event.Data = strings.TrimSpace(line[5:])
			case line == "":
				if event.Data != "" || event.Type != "" {
					ch <- event
					event = SSEEvent{}
				}
			}
		}
	}()
	return ch
}

// CollectSSE is a test helper that reads all SSE events from r synchronously.
func CollectSSE(r io.Reader) []SSEEvent {
	var out []SSEEvent
	for e := range parseSSE(r) {
		out = append(out, e)
	}
	return out
}

// bytesReader wraps a byte slice in a ReadCloser with known ContentLength.
func bytesReader(b []byte) *bytesReadCloser {
	return &bytesReadCloser{data: b}
}

type bytesReadCloser struct {
	data []byte
	pos  int
}

func (b *bytesReadCloser) Read(p []byte) (int, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	n := copy(p, b.data[b.pos:])
	b.pos += n
	return n, nil
}

func (b *bytesReadCloser) Close() error { return nil }

func readBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	return body, nil
}

// NewTransport creates the appropriate Transport for the given transport string.
// transport must be "auto", "sse", or "http".
func NewTransport(baseURL string, headers http.Header, transport string) (Transport, error) {
	client := &http.Client{Timeout: httpClientTimout}
	switch transport {
	case "http":
		return &StreamableHTTP{baseURL: baseURL, headers: headers, client: client}, nil
	case "sse":
		return &SSETransport{baseURL: baseURL, headers: headers, client: client}, nil
	case "auto", "":
		return &AutoTransport{baseURL: baseURL, headers: headers, client: client}, nil
	default:
		return nil, fmt.Errorf("unknown transport %q: must be auto, sse, or http", transport)
	}
}

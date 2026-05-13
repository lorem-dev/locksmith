package mcp_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorem-dev/locksmith/internal/mcp"
)

// mockTransport records sent messages, allows injecting server messages,
// and counts Connect invocations so tests can assert lazy-fetch behaviour.
type mockTransport struct {
	sent          [][]byte
	server        chan []byte
	connectCalled int
}

func newMockTransport() *mockTransport {
	return &mockTransport{server: make(chan []byte, 8)}
}

func (m *mockTransport) Connect(_ context.Context) (<-chan []byte, error) {
	m.connectCalled++
	return m.server, nil
}

func (m *mockTransport) Send(_ context.Context, msg []byte) error {
	cp := make([]byte, len(msg))
	copy(cp, msg)
	m.sent = append(m.sent, cp)
	return nil
}

func (m *mockTransport) Close() error { return nil }

func TestRunProxy_ForwardsMessages(t *testing.T) {
	cfg := mcp.ProxyConfig{
		URL:       "https://example.com",
		Transport: "http",
	}

	mock := newMockTransport()
	// Inject a server notification and close the channel so runLoop drains cleanly.
	serverMsg := []byte(`{"jsonrpc":"2.0","method":"notify","params":{}}`)
	mock.server <- serverMsg
	close(mock.server)

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	var stdout bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mcp.RunProxyWithTransport(ctx, cfg, mock, stdin, &stdout)
	require.NoError(t, err)

	require.Len(t, mock.sent, 1)
	assert.Contains(t, string(mock.sent[0]), `"method":"ping"`)

	assert.Contains(t, stdout.String(), `"method":"notify"`)
}

func TestRunProxy_LazyFetch_NoStdin_NoFetch(t *testing.T) {
	cfg := mcp.ProxyConfig{URL: "https://example.com", Transport: "http"}
	mock := newMockTransport()
	close(mock.server) // server side immediately closed

	stdin := strings.NewReader("") // immediate EOF, no client messages
	var stdout bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mcp.RunProxyWithTransport(ctx, cfg, mock, stdin, &stdout)
	require.NoError(t, err)

	assert.Equal(t, 0, mock.connectCalled, "Connect must not be called when stdin is empty")
	assert.Empty(t, mock.sent, "Send must not be called when stdin is empty")
	assert.Empty(t, stdout.String())
}

func TestRunProxy_LazyFetch_SkipsEmptyLinesBeforeFirstMessage(t *testing.T) {
	cfg := mcp.ProxyConfig{URL: "https://example.com", Transport: "http"}
	mock := newMockTransport()
	close(mock.server)

	// Leading blank lines must not trigger Connect.
	stdin := strings.NewReader("\n\n\n" + `{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	var stdout bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mcp.RunProxyWithTransport(ctx, cfg, mock, stdin, &stdout)
	require.NoError(t, err)

	assert.Equal(t, 1, mock.connectCalled, "Connect runs exactly once on the first non-empty line")
	require.Len(t, mock.sent, 1)
	assert.Contains(t, string(mock.sent[0]), `"method":"ping"`)
}

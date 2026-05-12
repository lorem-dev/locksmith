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

// mockTransport records sent messages and allows injecting server messages.
type mockTransport struct {
	sent   [][]byte
	server chan []byte
}

func newMockTransport() *mockTransport {
	return &mockTransport{server: make(chan []byte, 8)}
}

func (m *mockTransport) Connect(_ context.Context) (<-chan []byte, error) {
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

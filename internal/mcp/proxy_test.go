package mcp_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
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

func TestRunProxy_LazyAuth_TemplatedHeadersDeferred(t *testing.T) {
	fetcher := staticFetcher{"k": "tok"}
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		seen = append(seen, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	cfg := mcp.ProxyConfig{
		URL:       srv.URL,
		Transport: "http",
		Headers: []mcp.HeaderMapping{
			{Name: "Authorization", Template: "Bearer {key:k}"},
		},
	}

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	var stdout bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NoError(t, mcp.RunProxy(ctx, fetcher, cfg, stdin, &stdout))

	require.Len(t, seen, 2, "first request without auth, retry with auth")
	assert.Empty(t, seen[0])
	assert.Equal(t, "Bearer tok", seen[1])
}

func TestRunProxy_LazyAuth_StaticHeadersAlwaysSent(t *testing.T) {
	fetcher := staticFetcher{"k": "tok"}
	var ua, auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		ua = append(ua, r.Header.Get("X-Static-Header"))
		auth = append(auth, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	cfg := mcp.ProxyConfig{
		URL:       srv.URL,
		Transport: "http",
		Headers: []mcp.HeaderMapping{
			{Name: "X-Static-Header", Template: "always-here"},
			{Name: "Authorization", Template: "Bearer {key:k}"},
		},
	}

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	var stdout bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	require.NoError(t, mcp.RunProxy(ctx, fetcher, cfg, stdin, &stdout))

	require.Len(t, ua, 2)
	assert.Equal(t, "always-here", ua[0], "static header present on the first (unauthenticated) attempt")
	assert.Equal(t, "always-here", ua[1], "static header still present on the retry")
	require.Len(t, auth, 2)
	assert.Empty(t, auth[0])
	assert.Equal(t, "Bearer tok", auth[1])
}

func TestRunProxy_LazyAuth_NoTemplates_NoResolver(t *testing.T) {
	var fetcherCalls int
	fetcher := countingFetcher{inner: staticFetcher{}, calls: &fetcherCalls}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusUnauthorized) // server rejects, but no resolver to consult
	}))
	defer srv.Close()

	cfg := mcp.ProxyConfig{
		URL:       srv.URL,
		Transport: "http",
		Headers: []mcp.HeaderMapping{
			{Name: "X-Static-Header", Template: "always-here"},
		},
	}

	stdin := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"ping"}` + "\n")
	var stdout bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := mcp.RunProxy(ctx, fetcher, cfg, stdin, &stdout)
	require.Error(t, err)
	assert.Equal(t, 0, fetcherCalls, "no templated headers => fetcher never called")
}

// countingFetcher wraps another SecretFetcher and increments calls on
// every Fetch invocation.
type countingFetcher struct {
	inner mcp.SecretFetcher
	calls *int
}

func (f countingFetcher) Fetch(ctx context.Context, ref mcp.SecretRef) (string, error) {
	*f.calls++
	return f.inner.Fetch(ctx, ref)
}

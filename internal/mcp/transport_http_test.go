package mcp

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamableHTTP_JSONResponse(t *testing.T) {
	var received []byte
	response := []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		received = body
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(response)
	}))
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, nil, "http")
	require.NoError(t, err)
	defer transport.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msgCh, err := transport.Connect(ctx)
	require.NoError(t, err)

	msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	require.NoError(t, transport.Send(ctx, msg))

	select {
	case got := <-msgCh:
		assert.Equal(t, response, got)
	case <-ctx.Done():
		t.Fatal("timeout")
	}
	assert.Equal(t, msg, received)
}

func TestStreamableHTTP_SSEResponse(t *testing.T) {
	events := []string{
		`{"jsonrpc":"2.0","id":1,"result":{"part":1}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"part":2}}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, e := range events {
			fmt.Fprintf(w, "data: %s\n\n", e)
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, nil, "http")
	require.NoError(t, err)
	defer transport.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	msgCh, err := transport.Connect(ctx)
	require.NoError(t, err)
	require.NoError(t, transport.Send(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))

	var got []string
	for i := 0; i < len(events); i++ {
		select {
		case msg := <-msgCh:
			got = append(got, string(msg))
		case <-ctx.Done():
			t.Fatalf("timeout after %d messages", i)
		}
	}
	assert.Equal(t, events, got)
}

func TestAutoTransport_FallsBackToSSE(t *testing.T) {
	var received []byte
	notification := []byte(`{"jsonrpc":"2.0","method":"notify"}`)

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: endpoint\ndata: /messages\n\n")
		fmt.Fprintf(w, "data: %s\n\n", notification)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(50 * time.Millisecond)
	})
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received = body
		w.WriteHeader(http.StatusAccepted)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, nil, "auto")
	require.NoError(t, err)
	defer transport.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	msgCh, err := transport.Connect(ctx)
	require.NoError(t, err)

	msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)
	require.NoError(t, transport.Send(ctx, msg))

	select {
	case got := <-msgCh:
		assert.Equal(t, notification, got)
	case <-ctx.Done():
		t.Fatal("timeout waiting for server message")
	}
	assert.Equal(t, msg, received)
}

func TestStreamableHTTP_LazyAuth_200_StaysUnauthenticated(t *testing.T) {
	var resolveCalls int
	resolver := func(_ context.Context) (http.Header, error) {
		resolveCalls++
		return http.Header{"Authorization": []string{"Bearer tok"}}, nil
	}
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		seen = append(seen, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, newAuthState(resolver), "http")
	require.NoError(t, err)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = transport.Connect(ctx)
	require.NoError(t, err)
	require.NoError(t, transport.Send(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))
	require.NoError(t, transport.Send(ctx, []byte(`{"jsonrpc":"2.0","id":2,"method":"ping"}`)))

	assert.Equal(t, 0, resolveCalls, "resolver must not be called when no 401/403 ever arrives")
	assert.Equal(t, []string{"", ""}, seen, "both requests must go without an Authorization header")
}

func TestStreamableHTTP_LazyAuth_NoResolver_NoRetry(t *testing.T) {
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		posts++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, nil, "http")
	require.NoError(t, err)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = transport.Connect(ctx)
	require.NoError(t, err)
	err = transport.Send(ctx, []byte(`{}`))
	require.Error(t, err)
	assert.Equal(t, 1, posts, "nil resolver must not trigger a retry")
}

func TestStreamableHTTP_LazyAuth_401_TriggersResolveAndRetry(t *testing.T) {
	var resolveCalls int
	resolver := func(_ context.Context) (http.Header, error) {
		resolveCalls++
		return http.Header{"Authorization": []string{"Bearer tok"}}, nil
	}
	var seenAuth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		seenAuth = append(seenAuth, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, newAuthState(resolver), "http")
	require.NoError(t, err)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = transport.Connect(ctx)
	require.NoError(t, err)
	require.NoError(t, transport.Send(ctx, []byte(`{"jsonrpc":"2.0","id":1,"method":"ping"}`)))

	assert.Equal(t, 1, resolveCalls)
	require.Len(t, seenAuth, 2, "first request goes without auth, retry goes with auth")
	assert.Empty(t, seenAuth[0])
	assert.Equal(t, "Bearer tok", seenAuth[1])
}

func TestStreamableHTTP_LazyAuth_403_TreatedAsAuth(t *testing.T) {
	resolver := func(_ context.Context) (http.Header, error) {
		return http.Header{"Authorization": []string{"Bearer tok"}}, nil
	}
	var seenAuth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		seenAuth = append(seenAuth, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, newAuthState(resolver), "http")
	require.NoError(t, err)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = transport.Connect(ctx)
	require.NoError(t, err)
	require.NoError(t, transport.Send(ctx, []byte(`{}`)))
	require.Len(t, seenAuth, 2)
	assert.Empty(t, seenAuth[0])
	assert.Equal(t, "Bearer tok", seenAuth[1])
}

func TestStreamableHTTP_LazyAuth_AuthSticky(t *testing.T) {
	var resolveCalls int
	resolver := func(_ context.Context) (http.Header, error) {
		resolveCalls++
		return http.Header{"Authorization": []string{"Bearer tok"}}, nil
	}
	var seenAuth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		seenAuth = append(seenAuth, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, newAuthState(resolver), "http")
	require.NoError(t, err)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = transport.Connect(ctx)
	require.NoError(t, err)
	require.NoError(t, transport.Send(ctx, []byte(`{"id":1}`)))
	require.NoError(t, transport.Send(ctx, []byte(`{"id":2}`)))
	require.NoError(t, transport.Send(ctx, []byte(`{"id":3}`)))

	assert.Equal(t, 1, resolveCalls, "resolver must be invoked exactly once across the connection")
	require.Len(t, seenAuth, 4)
	assert.Empty(t, seenAuth[0])
	assert.Equal(t, "Bearer tok", seenAuth[1])
	assert.Equal(t, "Bearer tok", seenAuth[2])
	assert.Equal(t, "Bearer tok", seenAuth[3])
}

func TestStreamableHTTP_LazyAuth_AuthFailsAfterRetry(t *testing.T) {
	resolver := func(_ context.Context) (http.Header, error) {
		return http.Header{"Authorization": []string{"Bearer tok"}}, nil
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusUnauthorized) // always rejects
	}))
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, newAuthState(resolver), "http")
	require.NoError(t, err)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = transport.Connect(ctx)
	require.NoError(t, err)
	err = transport.Send(ctx, []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "401")
}

// TestStreamableHTTP_BodyResolveThen401_NoSecondResolve verifies that
// when the shared authState has already been resolved (e.g. by the
// body-level retry path in the proxy run loop), a subsequent 401 from
// the server is treated as a genuine failure: the transport does NOT
// invoke the resolver again, and the error is propagated.
func TestStreamableHTTP_BodyResolveThen401_NoSecondResolve(t *testing.T) {
	var calls atomic.Int32
	resolver := func(_ context.Context) (http.Header, error) {
		calls.Add(1)
		return http.Header{"X-Token": []string{"ok"}}, nil
	}
	auth := newAuthState(resolver)
	if err := auth.resolveOnce(context.Background()); err != nil {
		t.Fatalf("seed resolveOnce: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	tr := &StreamableHTTP{
		baseURL:       server.URL,
		client:        server.Client(),
		staticHeaders: http.Header{},
		auth:          auth,
	}
	_, err := tr.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	err = tr.Send(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"x"}`))
	if err == nil {
		t.Fatal("Send: expected 401 error, got nil")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("resolver calls = %d, want 1 (no second resolve)", got)
	}
}

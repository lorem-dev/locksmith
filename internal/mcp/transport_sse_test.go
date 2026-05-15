package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSSETransport_RoundTrip(t *testing.T) {
	var received []byte
	notification := []byte(`{"jsonrpc":"2.0","method":"notify","params":{}}`)

	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		fmt.Fprintf(w, "event: endpoint\ndata: /messages\n\n")
		fmt.Fprintf(w, "data: %s\n\n", notification)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(50 * time.Millisecond)
	})
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		received = body
		w.WriteHeader(http.StatusAccepted)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, nil, "sse")
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
		assert.Equal(t, notification, got)
	case <-ctx.Done():
		t.Fatal("timeout waiting for server message")
	}

	assert.Equal(t, msg, received)
}

func TestSSETransport_AuthHeader(t *testing.T) {
	var gotAuth string
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: endpoint\ndata: /messages\n\n")
		time.Sleep(50 * time.Millisecond)
	})
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	headers := http.Header{"Authorization": {"Bearer tok-123"}}
	transport, err := NewTransport(srv.URL, headers, nil, "sse")
	require.NoError(t, err)
	defer transport.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = transport.Connect(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Bearer tok-123", gotAuth)
}

func TestSSETransport_LazyAuth_Connect_401_ReopensWithAuth(t *testing.T) {
	var resolveCalls int
	resolver := func(_ context.Context) (http.Header, error) {
		resolveCalls++
		return http.Header{"Authorization": []string{"Bearer tok"}}, nil
	}
	var getCalls []string
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		getCalls = append(getCalls, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: endpoint\ndata: /messages\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(50 * time.Millisecond)
	})
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, newAuthState(resolver), "sse")
	require.NoError(t, err)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = transport.Connect(ctx)
	require.NoError(t, err)

	assert.Equal(t, 1, resolveCalls)
	require.Len(t, getCalls, 2)
	assert.Empty(t, getCalls[0])
	assert.Equal(t, "Bearer tok", getCalls[1])
}

func TestSSETransport_LazyAuth_Send_401_RetriesWithAuth(t *testing.T) {
	var resolveCalls int
	resolver := func(_ context.Context) (http.Header, error) {
		resolveCalls++
		return http.Header{"Authorization": []string{"Bearer tok"}}, nil
	}
	var getCalls int
	var postCalls []string
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", func(w http.ResponseWriter, r *http.Request) {
		getCalls++
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "event: endpoint\ndata: /messages\n\n")
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		time.Sleep(50 * time.Millisecond)
	})
	mux.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		postCalls = append(postCalls, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	transport, err := NewTransport(srv.URL, nil, newAuthState(resolver), "sse")
	require.NoError(t, err)
	defer transport.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = transport.Connect(ctx)
	require.NoError(t, err)
	require.NoError(t, transport.Send(ctx, []byte(`{}`)))

	assert.Equal(t, 1, resolveCalls)
	assert.Equal(t, 1, getCalls, "GET /sse must NOT be reopened after late auth (documented limitation)")
	require.Len(t, postCalls, 2)
	assert.Empty(t, postCalls[0])
	assert.Equal(t, "Bearer tok", postCalls[1])
}

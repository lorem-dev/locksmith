package mcp_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorem-dev/locksmith/internal/mcp"
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

	transport, err := mcp.NewTransport(srv.URL, nil, nil, "sse")
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
	transport, err := mcp.NewTransport(srv.URL, headers, nil, "sse")
	require.NoError(t, err)
	defer transport.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err = transport.Connect(ctx)
	require.NoError(t, err)
	assert.Equal(t, "Bearer tok-123", gotAuth)
}

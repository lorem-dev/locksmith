package mcp_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lorem-dev/locksmith/internal/mcp"
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

	transport, err := mcp.NewTransport(srv.URL, nil, "http")
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

	transport, err := mcp.NewTransport(srv.URL, nil, "http")
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

	transport, err := mcp.NewTransport(srv.URL, nil, "auto")
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

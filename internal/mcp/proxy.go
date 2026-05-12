package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// HeaderMapping maps an HTTP header name to a value template.
// The template supports {key:alias} and {vault:name path:value} tokens.
type HeaderMapping struct {
	Name     string
	Template string
}

// ProxyConfig holds the configuration for RunProxy.
type ProxyConfig struct {
	URL       string
	Transport string // "auto", "sse", or "http"
	Headers   []HeaderMapping
}

// RunProxy resolves secrets, builds HTTP headers, creates the transport,
// and runs the stdio<->HTTP proxy loop.
func RunProxy(ctx context.Context, fetcher SecretFetcher, cfg ProxyConfig, in io.Reader, out io.Writer) error {
	headers, err := resolveHeaders(ctx, fetcher, cfg.Headers)
	if err != nil {
		return err
	}
	transport, err := NewTransport(cfg.URL, headers, cfg.Transport)
	if err != nil {
		return err
	}
	return RunProxyWithTransport(ctx, cfg, transport, in, out)
}

// RunProxyWithTransport runs the proxy loop with a pre-built transport.
// Exposed for testing with mock transports. The transport's message channel
// must be closed when the server disconnects for runLoop to drain and return.
func RunProxyWithTransport(ctx context.Context, cfg ProxyConfig, transport Transport, in io.Reader, out io.Writer) error {
	msgCh, err := transport.Connect(ctx)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", cfg.URL, err)
	}
	return runLoop(ctx, transport, msgCh, in, out)
}

func resolveHeaders(ctx context.Context, fetcher SecretFetcher, mappings []HeaderMapping) (http.Header, error) {
	headers := make(http.Header)
	for _, h := range mappings {
		value, err := ResolveTemplate(ctx, h.Template, fetcher)
		if err != nil {
			return nil, fmt.Errorf("resolving header %s: %w", h.Name, err)
		}
		headers.Set(h.Name, value)
	}
	return headers, nil
}

func runLoop(ctx context.Context, transport Transport, msgCh <-chan []byte, in io.Reader, out io.Writer) error {
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	writeMsg := func(msg []byte) {
		mu.Lock()
		defer mu.Unlock()
		out.Write(append(msg, '\n')) //nolint:errcheck
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range msgCh {
			writeMsg(msg)
		}
	}()

	scanner := bufio.NewScanner(in)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := transport.Send(ctx, line); err != nil {
			wg.Wait()
			return fmt.Errorf("sending message: %w", err)
		}
	}
	wg.Wait()
	return scanner.Err()
}

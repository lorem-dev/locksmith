package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/lorem-dev/locksmith/internal/log"
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
	log.Debug().
		Str("url", RedactURL(cfg.URL)).
		Str("transport", cfg.Transport).
		Int("header_injections", len(cfg.Headers)).
		Msg("mcp proxy: starting")
	headers, err := resolveHeaders(ctx, fetcher, cfg.Headers)
	if err != nil {
		log.Debug().Err(err).Msg("mcp proxy: header resolution failed")
		return err
	}
	transport, err := NewTransport(cfg.URL, headers, cfg.Transport)
	if err != nil {
		log.Debug().Err(err).Msg("mcp proxy: creating transport failed")
		return err
	}
	log.Debug().Msg("mcp proxy: transport created, entering proxy loop")
	return RunProxyWithTransport(ctx, cfg, transport, in, out)
}

// RunProxyWithTransport runs the proxy loop with a pre-built transport.
// Exposed for testing with mock transports. The transport's message channel
// must be closed when the server disconnects for runLoop to drain and return.
func RunProxyWithTransport(
	ctx context.Context,
	cfg ProxyConfig,
	transport Transport,
	in io.Reader,
	out io.Writer,
) error {
	log.Debug().Str("url", RedactURL(cfg.URL)).Msg("mcp proxy: connecting")
	msgCh, err := transport.Connect(ctx)
	if err != nil {
		log.Debug().Err(err).Msg("mcp proxy: connect failed")
		return fmt.Errorf("connecting to %s: %w", RedactURL(cfg.URL), err)
	}
	log.Debug().Msg("mcp proxy: connected; starting stdio loop")
	return runLoop(ctx, transport, msgCh, in, out)
}

func resolveHeaders(ctx context.Context, fetcher SecretFetcher, mappings []HeaderMapping) (http.Header, error) {
	headers := make(http.Header)
	for _, h := range mappings {
		log.Debug().Str("header", h.Name).Msg("mcp proxy: resolving header")
		value, err := ResolveTemplate(ctx, h.Template, fetcher)
		if err != nil {
			log.Debug().Err(err).Str("header", h.Name).Msg("mcp proxy: header resolution failed")
			return nil, fmt.Errorf("resolving header %s: %w", h.Name, err)
		}
		log.Debug().Str("header", h.Name).Int("len", len(value)).Msg("mcp proxy: header resolved")
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

	var serverMsgCount uint64
	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range msgCh {
			serverMsgCount++
			log.Debug().Uint64("seq", serverMsgCount).Int("len", len(msg)).Msg("mcp proxy: server -> client")
			writeMsg(msg)
		}
		log.Debug().Uint64("count", serverMsgCount).Msg("mcp proxy: server channel closed")
	}()

	scanner := bufio.NewScanner(in)
	var clientMsgCount uint64
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		clientMsgCount++
		log.Debug().Uint64("seq", clientMsgCount).Int("len", len(line)).Msg("mcp proxy: client -> server")
		if err := transport.Send(ctx, line); err != nil {
			log.Debug().Err(err).Uint64("seq", clientMsgCount).Msg("mcp proxy: transport.Send failed")
			wg.Wait()
			return fmt.Errorf("sending message: %w", err)
		}
	}
	log.Debug().Uint64("count", clientMsgCount).Msg("mcp proxy: stdin scanner exited")
	wg.Wait()
	if err := scanner.Err(); err != nil {
		log.Debug().Err(err).Msg("mcp proxy: stdin scanner error")
		return fmt.Errorf("reading stdin: %w", err)
	}
	return nil
}

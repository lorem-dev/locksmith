package mcp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
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

// transportSetup is called on the first non-empty stdin line. It is
// responsible for resolving any deferred work (header secret lookups,
// transport construction, Connect) and returning the transport plus
// its server-message channel.
type transportSetup func(ctx context.Context) (Transport, <-chan []byte, error)

// RunProxy resolves secrets lazily, builds the transport on the first
// client message, then forwards stdio<->HTTP for the lifetime of the
// connection.
func RunProxy(ctx context.Context, fetcher SecretFetcher, cfg ProxyConfig, in io.Reader, out io.Writer) error {
	log.Debug().
		Str("url", RedactURL(cfg.URL)).
		Str("transport", cfg.Transport).
		Int("header_injections", len(cfg.Headers)).
		Msg("mcp proxy: starting (lazy)")
	setup := func(ctx context.Context) (Transport, <-chan []byte, error) {
		headers, err := resolveHeaders(ctx, fetcher, cfg.Headers)
		if err != nil {
			log.Debug().Err(err).Msg("mcp proxy: header resolution failed")
			return nil, nil, err
		}
		transport, err := NewTransport(cfg.URL, headers, cfg.Transport)
		if err != nil {
			log.Debug().Err(err).Msg("mcp proxy: creating transport failed")
			return nil, nil, err
		}
		msgCh, err := transport.Connect(ctx)
		if err != nil {
			log.Debug().Err(err).Msg("mcp proxy: connect failed")
			return nil, nil, fmt.Errorf("connecting to %s: %w", RedactURL(cfg.URL), err)
		}
		return transport, msgCh, nil
	}
	return runLoop(ctx, setup, in, out)
}

// RunProxyWithTransport runs the proxy loop against a pre-constructed
// Transport. Used by tests that inject a mock. The transport's Connect
// is still deferred to the first non-empty stdin line so the lazy
// contract is identical to RunProxy.
func RunProxyWithTransport(
	ctx context.Context,
	cfg ProxyConfig,
	transport Transport,
	in io.Reader,
	out io.Writer,
) error {
	setup := func(ctx context.Context) (Transport, <-chan []byte, error) {
		msgCh, err := transport.Connect(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("connecting to %s: %w", RedactURL(cfg.URL), err)
		}
		return transport, msgCh, nil
	}
	return runLoop(ctx, setup, in, out)
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

func runLoop(ctx context.Context, setup transportSetup, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)

	firstLine, err := readFirstNonEmptyLine(reader)
	if err != nil {
		return err
	}
	if firstLine == nil {
		// EOF before any non-empty line; nothing to proxy.
		return nil
	}

	transport, msgCh, err := setup(ctx)
	if err != nil {
		return err
	}
	log.Debug().Msg("mcp proxy: connected; starting stdio loop")

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

	var clientMsgCount uint64
	if err := sendClientMessage(ctx, transport, firstLine, &clientMsgCount); err != nil {
		wg.Wait()
		return err
	}

	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimRight(line, "\r\n")
			if len(trimmed) > 0 {
				if err := sendClientMessage(ctx, transport, trimmed, &clientMsgCount); err != nil {
					wg.Wait()
					return err
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			wg.Wait()
			return fmt.Errorf("reading stdin: %w", readErr)
		}
	}
	log.Debug().Uint64("count", clientMsgCount).Msg("mcp proxy: stdin scanner exited")
	wg.Wait()
	return nil
}

// readFirstNonEmptyLine consumes empty lines from reader and returns
// the first line that contains non-whitespace content (without trailing
// CR/LF). Returns (nil, nil) on EOF before any non-empty line.
func readFirstNonEmptyLine(reader *bufio.Reader) ([]byte, error) {
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimRight(line, "\r\n")
			if len(trimmed) > 0 {
				return trimmed, nil
			}
		}
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
	}
}

func sendClientMessage(ctx context.Context, transport Transport, line []byte, count *uint64) error {
	*count++
	log.Debug().Uint64("seq", *count).Int("len", len(line)).Msg("mcp proxy: client -> server")
	if err := transport.Send(ctx, line); err != nil {
		log.Debug().Err(err).Uint64("seq", *count).Msg("mcp proxy: transport.Send failed")
		return fmt.Errorf("sending message: %w", err)
	}
	return nil
}

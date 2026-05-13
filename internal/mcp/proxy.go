package mcp

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	static, templates := splitHeaders(cfg.Headers)
	resolver := buildAuthResolver(fetcher, templates)
	log.Debug().
		Str("url", RedactURL(cfg.URL)).
		Str("transport", cfg.Transport).
		Int("static_headers", len(static)).
		Int("templated_headers", len(templates)).
		Msg("mcp proxy: starting (lazy auth)")

	setup := func(ctx context.Context) (Transport, <-chan []byte, error) {
		transport, err := NewTransport(cfg.URL, static, resolver, cfg.Transport)
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

// splitHeaders partitions cfg.Headers by whether the template contains a
// vault reference token. Static entries are pre-built into a full
// http.Header; templated entries are returned unchanged for lazy
// resolution.
func splitHeaders(mappings []HeaderMapping) (http.Header, []HeaderMapping) {
	static := make(http.Header)
	var templates []HeaderMapping
	for _, h := range mappings {
		if strings.Contains(h.Template, "{") {
			templates = append(templates, h)
			continue
		}
		static.Set(h.Name, h.Template)
	}
	return static, templates
}

// buildAuthResolver returns a HeaderResolver closure that resolves the
// supplied templated headers via fetcher on first call. Returns nil if
// templates is empty - signalling to NewTransport that no auth-retry is
// possible for this proxy session.
func buildAuthResolver(fetcher SecretFetcher, templates []HeaderMapping) HeaderResolver {
	if len(templates) == 0 {
		return nil
	}
	return func(ctx context.Context) (http.Header, error) {
		log.Debug().Int("count", len(templates)).Msg("mcp proxy: resolving deferred auth headers")
		headers := make(http.Header, len(templates))
		for _, h := range templates {
			value, err := ResolveTemplate(ctx, h.Template, fetcher)
			if err != nil {
				log.Debug().Err(err).Str("header", h.Name).Msg("mcp proxy: deferred header resolution failed")
				return nil, fmt.Errorf("resolving header %s: %w", h.Name, err)
			}
			headers.Set(h.Name, value)
		}
		log.Debug().Int("count", len(headers)).Msg("mcp proxy: deferred auth headers resolved")
		return headers, nil
	}
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

	done := make(chan struct{})
	shutdown := func() {
		select {
		case <-done:
		default:
			close(done)
		}
		transport.Close() //nolint:errcheck // best-effort shutdown
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		forwardServerMessages(msgCh, done, writeMsg)
	}()

	var clientMsgCount uint64
	if err := sendClientMessage(ctx, transport, firstLine, &clientMsgCount); err != nil {
		shutdown()
		wg.Wait()
		return err
	}

	for {
		line, readErr := reader.ReadBytes('\n')
		if len(line) > 0 {
			trimmed := bytes.TrimRight(line, "\r\n")
			if len(trimmed) > 0 {
				if err := sendClientMessage(ctx, transport, trimmed, &clientMsgCount); err != nil {
					shutdown()
					wg.Wait()
					return err
				}
			}
		}
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			shutdown()
			wg.Wait()
			return fmt.Errorf("reading stdin: %w", readErr)
		}
	}
	log.Debug().Uint64("count", clientMsgCount).Msg("mcp proxy: stdin scanner exited")
	shutdown()
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

// forwardServerMessages reads server messages from msgCh and dispatches
// them to writeMsg until either msgCh closes or done fires. When done
// fires it first drains any messages already buffered in msgCh so late
// responses still reach the client before shutdown.
func forwardServerMessages(msgCh <-chan []byte, done <-chan struct{}, writeMsg func([]byte)) {
	var count uint64
	forward := func(msg []byte) {
		count++
		log.Debug().Uint64("seq", count).Int("len", len(msg)).Msg("mcp proxy: server -> client")
		writeMsg(msg)
	}
	for {
		select {
		case msg, ok := <-msgCh:
			if !ok {
				log.Debug().Uint64("count", count).Msg("mcp proxy: server channel closed")
				return
			}
			forward(msg)
		case <-done:
			for {
				select {
				case msg, ok := <-msgCh:
					if !ok {
						log.Debug().Uint64("count", count).Msg("mcp proxy: server channel closed during drain")
						return
					}
					forward(msg)
				default:
					log.Debug().Uint64("count", count).Msg("mcp proxy: server reader stopped")
					return
				}
			}
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

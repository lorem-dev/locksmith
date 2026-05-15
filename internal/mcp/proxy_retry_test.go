package mcp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// scriptedTransport is a Transport that records sent messages and lets
// the test push pre-canned server responses on demand. Suitable for
// driving the proxy run loop without a real HTTP server.
type scriptedTransport struct {
	mu      sync.Mutex
	sent    [][]byte
	out     chan []byte
	done    chan struct{}
	sendErr error
	closed  bool
}

func newScriptedTransport() *scriptedTransport {
	return &scriptedTransport{out: make(chan []byte, 16), done: make(chan struct{})}
}

func (t *scriptedTransport) Connect(_ context.Context) (<-chan []byte, error) {
	return t.out, nil
}

func (t *scriptedTransport) Send(_ context.Context, msg []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.sendErr != nil {
		return t.sendErr
	}
	t.sent = append(t.sent, append([]byte(nil), msg...))
	return nil
}

func (t *scriptedTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		t.closed = true
		close(t.done)
		close(t.out)
	}
	return nil
}

func (t *scriptedTransport) push(msg string) { t.out <- []byte(msg) }

func (t *scriptedTransport) sentCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.sent)
}

// delayedEOFReader emits the bytes of `body` and then blocks subsequent
// reads on `wait` so the run loop does not observe stdin EOF until the
// test goroutine signals completion (typically by closing `wait` after
// tr.Close() has been called). This lets scripted tests push server
// responses and trigger retries without racing the shutdown path.
type delayedEOFReader struct {
	body *strings.Reader
	wait <-chan struct{}
}

func (r *delayedEOFReader) Read(p []byte) (int, error) {
	if r.body.Len() > 0 {
		return r.body.Read(p)
	}
	<-r.wait
	return 0, io.EOF
}

// runProxyForTest drives runLoop with the given scripted transport and
// resolver, then returns the non-empty lines written to stdout. stdin
// is kept open until tr is closed so the scripted server goroutine can
// push responses without racing shutdown.
func runProxyForTest(
	t *testing.T,
	tr *scriptedTransport,
	resolver HeaderResolver,
	stdin string,
) ([][]byte, *authState) {
	t.Helper()
	auth := newAuthState(resolver)
	var stdout bytes.Buffer
	setup := func(ctx context.Context) (Transport, <-chan []byte, error) {
		ch, _ := tr.Connect(ctx)
		return tr, ch, nil
	}
	reader := &delayedEOFReader{body: strings.NewReader(stdin), wait: tr.done}
	if err := runLoop(context.Background(), setup, auth, reader, &stdout); err != nil {
		t.Fatalf("runLoop: %v", err)
	}
	var lines [][]byte
	for _, l := range bytes.Split(stdout.Bytes(), []byte{'\n'}) {
		if len(l) > 0 {
			lines = append(lines, l)
		}
	}
	return lines, auth
}

func TestProxy_BodyError_TriggersResolveAndRetry(t *testing.T) {
	tr := newScriptedTransport()
	var calls atomic.Int32
	resolver := func(ctx context.Context) (http.Header, error) {
		calls.Add(1)
		h := http.Header{}
		h.Set("X-Token", "ok")
		return h, nil
	}
	go func() {
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[{"type":"text","text":"err"}]}}`)
		for tr.sentCount() < 2 {
			time.Sleep(5 * time.Millisecond)
		}
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":false,"content":[{"type":"text","text":"ok"}]}}`)
		tr.Close()
	}()
	stdin := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\"}\n"
	lines, auth := runProxyForTest(t, tr, resolver, stdin)
	if calls.Load() != 1 {
		t.Errorf("resolver calls = %d, want 1", calls.Load())
	}
	if !auth.Attempted() {
		t.Errorf("Attempted() = false")
	}
	if len(lines) != 1 {
		t.Fatalf("stdout lines = %d, want 1; got: %q", len(lines), lines)
	}
	if !bytes.Contains(lines[0], []byte(`"isError":false`)) {
		t.Errorf("client did not receive retry success: %q", lines[0])
	}
}

func TestProxy_BodyError_ResolveFails_ForwardsOriginal(t *testing.T) {
	tr := newScriptedTransport()
	want := errors.New("vault locked")
	resolver := func(ctx context.Context) (http.Header, error) { return nil, want }
	go func() {
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[{"type":"text","text":"err"}]}}`)
		tr.Close()
	}()
	stdin := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\"}\n"
	lines, auth := runProxyForTest(t, tr, resolver, stdin)
	if !auth.Attempted() {
		t.Errorf("Attempted() = false")
	}
	if len(lines) != 1 || !bytes.Contains(lines[0], []byte(`"isError":true`)) {
		t.Fatalf("stdout did not contain original error: %q", lines)
	}
}

func TestProxy_BodyError_RetrySendFails_ForwardsOriginal(t *testing.T) {
	tr := newScriptedTransport()
	resolver := func(ctx context.Context) (http.Header, error) {
		return http.Header{"X-Token": []string{"ok"}}, nil
	}
	go func() {
		// Wait for runLoop to send the original request before flipping
		// sendErr; otherwise the initial Send fails and the retry path
		// is not exercised.
		for tr.sentCount() < 1 {
			time.Sleep(5 * time.Millisecond)
		}
		tr.mu.Lock()
		tr.sendErr = errors.New("connect fail")
		tr.mu.Unlock()
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[{"type":"text","text":"err"}]}}`)
		time.Sleep(50 * time.Millisecond)
		tr.Close()
	}()
	stdin := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\"}\n"
	lines, auth := runProxyForTest(t, tr, resolver, stdin)
	if !auth.Attempted() {
		t.Errorf("Attempted() = false")
	}
	if len(lines) != 1 || !bytes.Contains(lines[0], []byte(`"isError":true`)) {
		t.Fatalf("stdout did not contain original error: %q", lines)
	}
}

func TestProxy_BodyError_RetryAlsoErrors_ForwardsRetryResponse(t *testing.T) {
	tr := newScriptedTransport()
	resolver := func(ctx context.Context) (http.Header, error) {
		return http.Header{"X-Token": []string{"ok"}}, nil
	}
	go func() {
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[{"type":"text","text":"err1"}]}}`)
		for tr.sentCount() < 2 {
			time.Sleep(5 * time.Millisecond)
		}
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[{"type":"text","text":"err2"}]}}`)
		tr.Close()
	}()
	stdin := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\"}\n"
	lines, _ := runProxyForTest(t, tr, resolver, stdin)
	if len(lines) != 1 || !bytes.Contains(lines[0], []byte(`"err2"`)) {
		t.Fatalf("client should see retry response only; got: %q", lines)
	}
}

func TestProxy_ConcurrentInFlight_RetriesCorrectId(t *testing.T) {
	tr := newScriptedTransport()
	resolver := func(ctx context.Context) (http.Header, error) {
		return http.Header{"X-Token": []string{"ok"}}, nil
	}
	go func() {
		tr.push(`{"jsonrpc":"2.0","id":2,"result":{"isError":true,"content":[{"type":"text","text":"e2"}]}}`)
		for tr.sentCount() < 3 {
			time.Sleep(5 * time.Millisecond)
		}
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":false,"content":[{"type":"text","text":"ok1"}]}}`)
		tr.push(`{"jsonrpc":"2.0","id":2,"result":{"isError":false,"content":[{"type":"text","text":"ok2"}]}}`)
		tr.Close()
	}()
	stdin := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\"}\n" +
		"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\"}\n"
	lines, _ := runProxyForTest(t, tr, resolver, stdin)
	if len(lines) != 2 {
		t.Fatalf("expected 2 forwarded responses, got %d: %q", len(lines), lines)
	}
	if !bytes.Contains(lines[0], []byte(`"id":1`)) || !bytes.Contains(lines[1], []byte(`"id":2`)) {
		t.Errorf("unexpected line order: %q", lines)
	}
	if !bytes.Contains(lines[1], []byte(`"ok2"`)) {
		t.Errorf("id=2 line should be retry success: %q", lines[1])
	}
	for _, l := range lines {
		if bytes.Contains(l, []byte(`"e2"`)) {
			t.Errorf("original error leaked: %q", l)
		}
	}
}

func TestProxy_OnlyFirstErrorTriggers(t *testing.T) {
	tr := newScriptedTransport()
	var calls atomic.Int32
	resolver := func(ctx context.Context) (http.Header, error) {
		calls.Add(1)
		return http.Header{"X-Token": []string{"ok"}}, nil
	}
	go func() {
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":false,"content":[]}}`)
		tr.push(`{"jsonrpc":"2.0","id":2,"result":{"isError":true,"content":[]}}`)
		for tr.sentCount() < 3 {
			time.Sleep(5 * time.Millisecond)
		}
		tr.push(`{"jsonrpc":"2.0","id":2,"result":{"isError":false,"content":[{"type":"text","text":"ok"}]}}`)
		tr.Close()
	}()
	stdin := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\"}\n" +
		"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\"}\n"
	lines, auth := runProxyForTest(t, tr, resolver, stdin)
	if calls.Load() != 1 {
		t.Errorf("resolver calls = %d, want 1", calls.Load())
	}
	if !auth.Attempted() {
		t.Errorf("Attempted() = false")
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 forwarded responses, got %d: %q", len(lines), lines)
	}
	if !bytes.Contains(lines[1], []byte(`"ok"`)) {
		t.Errorf("id=2 line should be retry success: %q", lines[1])
	}
}

func TestProxy_AfterAttempted_NoParse(t *testing.T) {
	tr := newScriptedTransport()
	var calls atomic.Int32
	resolver := func(ctx context.Context) (http.Header, error) {
		calls.Add(1)
		return http.Header{"X-Token": []string{"ok"}}, nil
	}
	go func() {
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[]}}`)
		for tr.sentCount() < 2 {
			time.Sleep(5 * time.Millisecond)
		}
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":false,"content":[]}}`)
		tr.push(`{"jsonrpc":"2.0","id":2,"result":{"isError":true,"content":[]}}`)
		tr.Close()
	}()
	stdin := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\"}\n" +
		"{\"jsonrpc\":\"2.0\",\"id\":2,\"method\":\"tools/call\"}\n"
	_, _ = runProxyForTest(t, tr, resolver, stdin)
	if got := calls.Load(); got != 1 {
		t.Errorf("resolver calls = %d, want 1", got)
	}
}

func TestProxy_Notification_NotTracked(t *testing.T) {
	tr := newScriptedTransport()
	resolver := func(ctx context.Context) (http.Header, error) {
		return http.Header{"X-Token": []string{"ok"}}, nil
	}
	go func() {
		tr.push(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{}}`)
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":true,"content":[]}}`)
		for tr.sentCount() < 2 {
			time.Sleep(5 * time.Millisecond)
		}
		tr.push(`{"jsonrpc":"2.0","id":1,"result":{"isError":false,"content":[]}}`)
		tr.Close()
	}()
	stdin := "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\"}\n"
	lines, _ := runProxyForTest(t, tr, resolver, stdin)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (notification + retry success), got %d: %q", len(lines), lines)
	}
}

package mcp

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
)

func TestAuthState_ResolveOnce_Single(t *testing.T) {
	var calls atomic.Int32
	resolver := func(ctx context.Context) (http.Header, error) {
		calls.Add(1)
		h := http.Header{}
		h.Set("Authorization", "Bearer x")
		return h, nil
	}
	s := newAuthState(resolver)

	if s.Attempted() {
		t.Fatal("Attempted() = true before first call")
	}
	if err := s.resolveOnce(context.Background()); err != nil {
		t.Fatalf("resolveOnce: %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("resolver calls = %d, want 1", calls.Load())
	}
	if !s.Attempted() {
		t.Errorf("Attempted() = false after first call")
	}
	if got := s.Headers().Get("Authorization"); got != "Bearer x" {
		t.Errorf("Headers().Get(Authorization) = %q, want %q", got, "Bearer x")
	}
}

func TestAuthState_ResolveOnce_Concurrent(t *testing.T) {
	var calls atomic.Int32
	resolver := func(ctx context.Context) (http.Header, error) {
		calls.Add(1)
		h := http.Header{}
		h.Set("X", "y")
		return h, nil
	}
	s := newAuthState(resolver)

	const numGoroutines = 16
	var wg sync.WaitGroup
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			_ = s.resolveOnce(context.Background())
		}()
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Errorf("resolver calls = %d, want 1", got)
	}
	if got := s.Headers().Get("X"); got != "y" {
		t.Errorf("Headers().Get(X) = %q, want %q", got, "y")
	}
}

func TestAuthState_ResolveOnce_ResolverFails(t *testing.T) {
	var calls atomic.Int32
	want := errors.New("boom")
	resolver := func(ctx context.Context) (http.Header, error) {
		calls.Add(1)
		return nil, want
	}
	s := newAuthState(resolver)

	if err := s.resolveOnce(context.Background()); !errors.Is(err, want) {
		t.Errorf("resolveOnce err = %v, want errors.Is(_, boom)", err)
	}
	if !s.Attempted() {
		t.Errorf("Attempted() = false after failed call")
	}
	if s.Headers() != nil {
		t.Errorf("Headers() != nil after failed call")
	}
	if err := s.resolveOnce(context.Background()); err != nil {
		t.Errorf("second resolveOnce err = %v, want nil", err)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("resolver calls = %d, want 1", got)
	}
}

func TestAuthState_NilResolver(t *testing.T) {
	s := newAuthState(nil)
	if err := s.resolveOnce(context.Background()); err != nil {
		t.Errorf("resolveOnce with nil resolver: %v", err)
	}
	if !s.Attempted() {
		t.Errorf("Attempted() = false")
	}
	if s.Headers() != nil {
		t.Errorf("Headers() != nil")
	}
}

package mcp

import (
	"context"
	"net/http"
	"sync"
)

// authState carries the shared auth-header resolution lifecycle between
// the proxy run loop (body-level error trigger) and the transports
// (HTTP 401/403 trigger). Resolution happens at most once per session;
// after the first attempt succeeds or fails, the result is cached and
// the resolver is never invoked again.
type authState struct {
	mu        sync.Mutex
	attempted bool
	headers   http.Header
	resolver  HeaderResolver
}

// newAuthState constructs an authState whose resolveOnce invokes
// resolver. A nil resolver makes resolveOnce a no-op that still marks
// the state as attempted (so callers do not loop).
func newAuthState(resolver HeaderResolver) *authState {
	return &authState{resolver: resolver}
}

// resolveOnce invokes the resolver under the mutex if it has not been
// invoked yet. After return, Attempted() reports true regardless of
// outcome. If the resolver returns an error, headers remain nil and
// the error is propagated; subsequent calls short-circuit without
// invoking resolver again.
func (s *authState) resolveOnce(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.attempted {
		return nil
	}
	s.attempted = true
	if s.resolver == nil {
		return nil
	}
	h, err := s.resolver(ctx)
	if err != nil {
		return err
	}
	s.headers = h
	return nil
}

// Headers returns the cached headers (may be nil before resolve or
// after a failed resolve). The returned http.Header must not be
// mutated by callers; it is shared across all subsequent requests.
func (s *authState) Headers() http.Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.headers
}

// Attempted reports whether resolveOnce has been called (success or
// failure). After it returns true, no further resolves will occur.
func (s *authState) Attempted() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attempted
}

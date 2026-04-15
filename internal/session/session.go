// Package session manages agent sessions: creation, TTL-based expiry,
// per-session secret caching, and secure memory wipe on invalidation.
package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Session represents an active agent session.
type Session struct {
	ID          string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	AllowedKeys []string // empty means all keys are allowed
	secrets     map[string][]byte
}

// Store is a thread-safe in-memory store for active sessions.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewStore creates a new empty session store.
func NewStore() *Store {
	return &Store{sessions: make(map[string]*Session)}
}

// Create allocates a new session with the given TTL and optional key allowlist.
func (s *Store) Create(ttl time.Duration, allowedKeys []string) (*Session, error) {
	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generating session ID: %w", err)
	}
	now := time.Now()
	sess := &Session{
		ID:          id,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
		AllowedKeys: allowedKeys,
		secrets:     make(map[string][]byte),
	}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	return sess, nil
}

// Get returns the session for id, or an error if not found or expired.
func (s *Store) Get(id string) (*Session, error) {
	s.mu.RLock()
	sess, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	if time.Now().After(sess.ExpiresAt) {
		// Acquire write lock to delete expired session.
		s.mu.Lock()
		// Re-check under write lock (another goroutine may have deleted it).
		if sess, stillPresent := s.sessions[id]; stillPresent {
			for key, secret := range sess.secrets {
				wipeBytes(secret)
				delete(sess.secrets, key)
			}
			delete(s.sessions, id)
		}
		s.mu.Unlock()
		return nil, fmt.Errorf("session %q expired", id)
	}
	return sess, nil
}

// Delete invalidates a session and wipes all cached secrets from memory.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[id]
	if !ok {
		return fmt.Errorf("session %q not found", id)
	}
	for key, secret := range sess.secrets {
		wipeBytes(secret)
		delete(sess.secrets, key)
	}
	delete(s.sessions, id)
	return nil
}

// CacheSecret stores a secret value bound to the given session and key name.
func (s *Store) CacheSecret(sessionID, keyName string, secret []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	sess.secrets[keyName] = secret
}

// GetCachedSecret retrieves a cached secret. Returns (nil, false) if the key
// is not cached, the session does not exist, or the key is not in the allowlist.
func (s *Store) GetCachedSecret(sessionID, keyName string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, false
	}
	if !sess.isKeyAllowed(keyName) {
		return nil, false
	}
	val, ok := sess.secrets[keyName]
	return val, ok
}

// List returns a snapshot of all active sessions (without cached secrets).
func (s *Store) List() []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		result = append(result, Session{
			ID:          sess.ID,
			CreatedAt:   sess.CreatedAt,
			ExpiresAt:   sess.ExpiresAt,
			AllowedKeys: sess.AllowedKeys,
		})
	}
	return result
}

// Cleanup removes expired sessions and returns the count removed.
func (s *Store) Cleanup() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	removed := 0
	now := time.Now()
	for id, sess := range s.sessions {
		if now.After(sess.ExpiresAt) {
			for key, secret := range sess.secrets {
				wipeBytes(secret)
				delete(sess.secrets, key)
			}
			delete(s.sessions, id)
			removed++
		}
	}
	return removed
}

func (sess *Session) isKeyAllowed(keyName string) bool {
	if len(sess.AllowedKeys) == 0 {
		return true
	}
	for _, k := range sess.AllowedKeys {
		if k == keyName {
			return true
		}
	}
	return false
}

func generateID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "ls_" + hex.EncodeToString(b), nil
}

// wipeBytes zeroes a byte slice to prevent secrets from lingering in memory.
func wipeBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

package session_test

import (
	"testing"
	"time"

	"github.com/lorem-dev/locksmith/internal/session"
)

func TestStore_Create(t *testing.T) {
	s := session.NewStore()
	sess, err := s.Create(3*time.Hour, nil)
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if len(sess.ID) != 67 { // "ls_" + 64 hex chars
		t.Errorf("ID length = %d, want 67", len(sess.ID))
	}
	if !sess.ExpiresAt.After(time.Now().Add(2*time.Hour + 59*time.Minute)) {
		t.Error("ExpiresAt too early")
	}
}

func TestStore_Create_WithAllowedKeys(t *testing.T) {
	s := session.NewStore()
	sess, err := s.Create(time.Hour, []string{"key1", "key2"})
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if len(sess.AllowedKeys) != 2 {
		t.Fatalf("AllowedKeys len = %d, want 2", len(sess.AllowedKeys))
	}
}

func TestStore_Get(t *testing.T) {
	s := session.NewStore()
	created, _ := s.Create(time.Hour, nil)
	got, err := s.Get(created.ID)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID = %q, want %q", got.ID, created.ID)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	s := session.NewStore()
	_, err := s.Get("ls_nonexistent")
	if err == nil {
		t.Fatal("Get() expected error for nonexistent session")
	}
}

func TestStore_Get_Expired(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(1*time.Millisecond, nil)
	time.Sleep(5 * time.Millisecond)
	_, err := s.Get(sess.ID)
	if err == nil {
		t.Fatal("Get() expected error for expired session")
	}
}

func TestStore_Delete(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(time.Hour, nil)
	if err := s.Delete(sess.ID); err != nil {
		t.Fatalf("Delete() error: %v", err)
	}
	_, err := s.Get(sess.ID)
	if err == nil {
		t.Fatal("Get() expected error after Delete()")
	}
}

func TestStore_CacheSecret(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(time.Hour, nil)
	s.CacheSecret(sess.ID, "github-token", []byte("ghp_secret123"))
	val, ok := s.GetCachedSecret(sess.ID, "github-token")
	if !ok {
		t.Fatal("GetCachedSecret() returned not ok")
	}
	if string(val) != "ghp_secret123" {
		t.Errorf("cached secret = %q, want %q", string(val), "ghp_secret123")
	}
}

func TestStore_CacheSecret_NotAllowed(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(time.Hour, []string{"allowed-key"})
	_ = sess
	_, ok := s.GetCachedSecret(sess.ID, "forbidden-key")
	if ok {
		t.Fatal("GetCachedSecret() should return not ok for non-allowed key")
	}
}

func TestStore_Delete_WipesSecrets(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(time.Hour, nil)
	secret := []byte("sensitive-data")
	s.CacheSecret(sess.ID, "key", secret)
	s.Delete(sess.ID)
	for i, b := range secret {
		if b != 0 {
			t.Errorf("secret byte[%d] = %d, want 0 (not wiped)", i, b)
		}
	}
}

func TestStore_List(t *testing.T) {
	s := session.NewStore()
	s.Create(time.Hour, nil)
	s.Create(time.Hour, []string{"k1"})
	list := s.List()
	if len(list) != 2 {
		t.Fatalf("List() len = %d, want 2", len(list))
	}
}

func TestStore_Cleanup(t *testing.T) {
	s := session.NewStore()
	s.Create(1*time.Millisecond, nil)
	s.Create(time.Hour, nil)
	time.Sleep(5 * time.Millisecond)
	removed := s.Cleanup()
	if removed != 1 {
		t.Errorf("Cleanup() removed %d, want 1", removed)
	}
	if len(s.List()) != 1 {
		t.Errorf("List() after cleanup = %d, want 1", len(s.List()))
	}
}

func TestStore_Cleanup_WipesSecrets(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(1*time.Millisecond, nil)
	secret := []byte("wipe-me")
	s.CacheSecret(sess.ID, "k", secret)
	time.Sleep(5 * time.Millisecond)
	s.Cleanup()
	for i, b := range secret {
		if b != 0 {
			t.Errorf("secret byte[%d] = %d after Cleanup, want 0", i, b)
		}
	}
}

func TestStore_Delete_NotFound(t *testing.T) {
	s := session.NewStore()
	err := s.Delete("ls_doesnotexist")
	if err == nil {
		t.Fatal("Delete() expected error for nonexistent session")
	}
}

func TestStore_CacheSecret_NoSession(t *testing.T) {
	s := session.NewStore()
	// Should not panic when session does not exist.
	s.CacheSecret("ls_nosuchsession", "key", []byte("val"))
}

func TestStore_GetCachedSecret_NoSession(t *testing.T) {
	s := session.NewStore()
	_, ok := s.GetCachedSecret("ls_nosuchsession", "key")
	if ok {
		t.Fatal("GetCachedSecret() should return not ok for missing session")
	}
}

func TestStore_GetCachedSecret_AllowedKey(t *testing.T) {
	s := session.NewStore()
	sess, _ := s.Create(time.Hour, []string{"my-key"})
	s.CacheSecret(sess.ID, "my-key", []byte("secret"))
	val, ok := s.GetCachedSecret(sess.ID, "my-key")
	if !ok {
		t.Fatal("GetCachedSecret() should return ok for allowed key")
	}
	if string(val) != "secret" {
		t.Errorf("cached secret = %q, want %q", string(val), "secret")
	}
}

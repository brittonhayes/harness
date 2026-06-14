package auth

import (
	"os"
	"path/filepath"
	"testing"
)

// withTempConfig points os.UserConfigDir at a temp dir so the test never touches
// the real ~/.config/vala/auth.json.
func withTempConfig(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	// UserConfigDir honors XDG_CONFIG_HOME on Linux and falls back to HOME
	// elsewhere; set both so the test is portable.
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("HOME", home)
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	withTempConfig(t)
	s, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v, want nil for missing file", err)
	}
	if len(s.Providers) != 0 {
		t.Fatalf("expected empty store, got %d entries", len(s.Providers))
	}
	if _, ok := s.Get("anthropic"); ok {
		t.Fatal("Get on empty store should report not found")
	}
}

func TestSetGetRemoveRoundTrip(t *testing.T) {
	withTempConfig(t)
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := Credential{Type: "api", Key: "sk-test", Model: "gpt-5"}
	if err := s.Set("openai", want); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// A fresh Load must see the persisted credential.
	reloaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := reloaded.Get("openai")
	if !ok {
		t.Fatal("credential not persisted")
	}
	if got != want {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got, want)
	}

	if err := reloaded.Remove("openai"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, ok := reloaded.Get("openai"); ok {
		t.Fatal("credential not removed")
	}
}

func TestSetWritesRestrictivePermissions(t *testing.T) {
	dir := withTempConfig(t)
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set("anthropic", Credential{Type: "api", Key: "sk"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "vala", "auth.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("auth.json perm = %o, want 600", perm)
	}
}

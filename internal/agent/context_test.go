package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempUserConfigDir(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", home)
	t.Setenv("HOME", home)
	dir, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadOperatorContextEmpty(t *testing.T) {
	tempUserConfigDir(t)
	if got := LoadOperatorContext(t.TempDir()); got != "" {
		t.Fatalf("expected empty operator context, got %q", got)
	}
}

func TestLoadOperatorContextProject(t *testing.T) {
	tempUserConfigDir(t)
	work := t.TempDir()
	want := "## Log sources\nauth -> Okta"
	if err := os.WriteFile(filepath.Join(work, OperatorContextFile), []byte(want+"\n\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoadOperatorContext(work); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLoadOperatorContextMergesGlobalThenProject(t *testing.T) {
	cfg := tempUserConfigDir(t)
	if err := os.MkdirAll(filepath.Join(cfg, "vala"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "vala", OperatorContextFile), []byte("GLOBAL"), 0o644); err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, OperatorContextFile), []byte("PROJECT"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadOperatorContext(work)
	gi, pi := strings.Index(got, "GLOBAL"), strings.Index(got, "PROJECT")
	if gi < 0 || pi < 0 || gi > pi {
		t.Fatalf("expected GLOBAL before PROJECT, got %q", got)
	}
}

func TestSystemPromptIncludesOperatorContext(t *testing.T) {
	with := huntPrompt("/work", []string{"read"}, 1, "CROWN JEWELS: billing db")
	if !strings.Contains(with, "Standing context") || !strings.Contains(with, "CROWN JEWELS: billing db") {
		t.Fatalf("system prompt missing standing context section:\n%s", with)
	}
	if without := huntPrompt("/work", []string{"read"}, 1, ""); strings.Contains(without, "Standing context") {
		t.Fatalf("empty context should not add a section")
	}
}

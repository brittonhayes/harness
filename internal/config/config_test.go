package config

import "testing"

func TestDefaultCompactionSettings(t *testing.T) {
	cfg := Default()
	if cfg.ContextWindow != 200000 {
		t.Errorf("ContextWindow = %d, want 200000", cfg.ContextWindow)
	}
	if cfg.AutoCompactThreshold != 0.80 {
		t.Errorf("AutoCompactThreshold = %v, want 0.80", cfg.AutoCompactThreshold)
	}
}

func TestLoadCompactionEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VALA_CONTEXT_WINDOW", "50000")
	t.Setenv("VALA_AUTO_COMPACT_THRESHOLD", "0.5")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ContextWindow != 50000 {
		t.Errorf("ContextWindow = %d, want 50000", cfg.ContextWindow)
	}
	if cfg.AutoCompactThreshold != 0.5 {
		t.Errorf("AutoCompactThreshold = %v, want 0.5", cfg.AutoCompactThreshold)
	}
}

func TestLoadCompactionEnvIgnoresGarbage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("VALA_CONTEXT_WINDOW", "not-a-number")
	t.Setenv("VALA_AUTO_COMPACT_THRESHOLD", "nope")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Malformed env values are ignored, leaving the defaults intact.
	if cfg.ContextWindow != 200000 {
		t.Errorf("ContextWindow = %d, want 200000 (default)", cfg.ContextWindow)
	}
	if cfg.AutoCompactThreshold != 0.80 {
		t.Errorf("AutoCompactThreshold = %v, want 0.80 (default)", cfg.AutoCompactThreshold)
	}
}

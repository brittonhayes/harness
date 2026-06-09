package config

import "testing"

// findServer returns the named MCP server from a config, or fails.
func findServer(t *testing.T, cfg Config, name string) MCPServer {
	t.Helper()
	for _, s := range cfg.MCP {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("mcp server %q not found in %+v", name, cfg.MCP)
	return MCPServer{}
}

func TestScannerSamplesDirRegistersLocalSource(t *testing.T) {
	t.Setenv("SCANNER_SAMPLES_DIR", "samples")
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s := findServer(t, cfg, "scanner")
	if s.Source != "local" || s.Dir != "samples" {
		t.Fatalf("scanner source = %+v, want local source with dir samples", s)
	}
}

func TestScannerMCPURLTakesPrecedenceOverSamplesDir(t *testing.T) {
	t.Setenv("SCANNER_MCP_URL", "https://x.scanner.dev/mcp")
	t.Setenv("SCANNER_SAMPLES_DIR", "samples")
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	s := findServer(t, cfg, "scanner")
	// The live URL wins; the local source is not also registered.
	if s.Source == "local" || s.URL == "" {
		t.Fatalf("expected live scanner to win, got %+v", s)
	}
	if got := len(cfg.MCP); got != 1 {
		t.Fatalf("expected a single scanner server, got %d", got)
	}
}

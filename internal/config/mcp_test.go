package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveMCPUpsertsByName adds a new server and replaces an existing one by
// name, preserving unrelated keys and never writing resolved secrets.
func TestSaveMCPUpsertsByName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".vala.json")
	seed := `{
  "model": "claude-custom",
  "mcp": [{"name": "scanner", "url": "https://old.scanner.dev/mcp", "api_key_env": "SCANNER_API_KEY"}]
}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	// Replace scanner (same name) and add wiz (stdio). The resolved APIKey must
	// never be persisted.
	if err := SaveMCP(dir, MCPServer{Name: "scanner", Transport: "http", URL: "https://new.scanner.dev/mcp", APIKeyEnv: "SCANNER_API_KEY", APIKey: "secret"}); err != nil {
		t.Fatalf("SaveMCP scanner: %v", err)
	}
	if err := SaveMCP(dir, MCPServer{Name: "wiz", Transport: "stdio", Command: "wiz-mcp", Args: []string{"serve"}, EnvPassthrough: []string{"WIZ_TOKEN"}}); err != nil {
		t.Fatalf("SaveMCP wiz: %v", err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Model != "claude-custom" {
		t.Errorf("unrelated key clobbered: model=%q", cfg.Model)
	}
	if len(cfg.MCP) != 2 {
		t.Fatalf("expected 2 servers (scanner replaced, wiz added), got %d", len(cfg.MCP))
	}
	var scanner, wiz *MCPServer
	for i := range cfg.MCP {
		switch cfg.MCP[i].Name {
		case "scanner":
			scanner = &cfg.MCP[i]
		case "wiz":
			wiz = &cfg.MCP[i]
		}
	}
	if scanner == nil || scanner.URL != "https://new.scanner.dev/mcp" {
		t.Fatalf("scanner not upserted: %+v", scanner)
	}
	if wiz == nil || wiz.Command != "wiz-mcp" || wiz.Transport != "stdio" {
		t.Fatalf("wiz not saved: %+v", wiz)
	}

	// Secrets must not be written to disk.
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "secret") {
		t.Errorf("resolved APIKey was persisted to .vala.json:\n%s", raw)
	}
}

// TestLoadResolvesEnvPassthrough resolves stdio env var names from the
// environment into Env, leaving unset names out.
func TestLoadResolvesEnvPassthrough(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".vala.json")
	seed := `{"mcp": [{"name": "wiz", "transport": "stdio", "command": "wiz-mcp", "env": ["WIZ_ID", "WIZ_MISSING"]}]}`
	if err := os.WriteFile(path, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WIZ_ID", "abc123")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.MCP) != 1 {
		t.Fatalf("expected 1 server, got %d", len(cfg.MCP))
	}
	env := cfg.MCP[0].Env
	if env["WIZ_ID"] != "abc123" {
		t.Errorf("WIZ_ID not resolved: %v", env)
	}
	if _, ok := env["WIZ_MISSING"]; ok {
		t.Errorf("unset env var should not be present: %v", env)
	}
}

func TestSaveMCPStripsInvisibleURLCharacters(t *testing.T) {
	dir := t.TempDir()
	rawURL := "https://mcp-dev.notion.com/mcp\u2060 \u2060"
	if err := SaveMCP(dir, MCPServer{Name: NotionSearchServerName, URL: rawURL, OAuth: true}); err != nil {
		t.Fatalf("SaveMCP: %v", err)
	}

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	srv, ok := cfg.NotionSearchServer()
	if !ok {
		t.Fatal("notion MCP server not saved")
	}
	if srv.URL != "https://mcp-dev.notion.com/mcp" {
		t.Fatalf("URL = %q", srv.URL)
	}
}

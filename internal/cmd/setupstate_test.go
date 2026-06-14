package cmd

import (
	"testing"

	"github.com/brittonhayes/vala/internal/config"
)

func TestEvidenceConfigured(t *testing.T) {
	if evidenceConfigured(config.Config{}) {
		t.Error("no MCP servers should report not configured")
	}
	cfg := config.Config{MCP: []config.MCPServer{{Name: "scanner"}}}
	if !evidenceConfigured(cfg) {
		t.Error("a configured MCP server should report configured")
	}
	onlyNotion := config.Config{MCP: []config.MCPServer{{Name: config.NotionSearchServerName}}}
	if evidenceConfigured(onlyNotion) {
		t.Error("the reserved Notion MCP server should not count as evidence")
	}
}

func TestProviderConfiguredFromEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	if !providerConfigured(config.Config{Provider: "anthropic"}) {
		t.Error("an API key in the environment should satisfy provider detection")
	}
}

func TestProviderConfiguredLocalNeedsNoKey(t *testing.T) {
	if !providerConfigured(config.Config{Provider: "ollama"}) {
		t.Error("a local provider needs no key and should report configured")
	}
}

func TestSetupCompleteRequiresAllThree(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	// Provider satisfied, but no brain and no evidence.
	if setupComplete(config.Config{Provider: "anthropic"}) {
		t.Error("setup should be incomplete without a brain and evidence")
	}
	cfg := config.Config{
		Provider:  "anthropic",
		BrainFile: ".vala/brain.json",
		MCP:       []config.MCPServer{{Name: "scanner"}},
	}
	if !setupComplete(cfg) {
		t.Error("provider + brain + evidence should be complete")
	}

	// A Notion brain that is configured but missing a store is not "ready": setup
	// must treat it as incomplete so it routes to repair rather than launching.
	partial := config.Config{
		Provider: "anthropic",
		MCP:      []config.MCPServer{{Name: "scanner"}},
		Notion:   fullNotion,
	}
	partial.Notion.Coverage = ""
	if setupComplete(partial) {
		t.Error("an incomplete Notion brain should make setup incomplete")
	}
	complete := partial
	complete.Notion = fullNotion
	if setupComplete(complete) {
		t.Error("a complete Notion brain without Notion MCP search should be incomplete")
	}
	complete.MCP = append(complete.MCP, config.MCPServer{Name: config.NotionSearchServerName, URL: config.DefaultNotionMCPURL, OAuth: true})
	if !setupComplete(complete) {
		t.Error("a complete Notion brain with Notion MCP search should satisfy setup")
	}
}

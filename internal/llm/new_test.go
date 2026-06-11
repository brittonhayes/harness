package llm

import (
	"errors"
	"testing"

	"github.com/brittonhayes/vala/internal/config"
)

func TestResolveModel(t *testing.T) {
	tests := []struct {
		name         string
		cfg          config.Config
		wantProvider string
		wantModel    string
	}{
		{
			name:         "explicit provider wins",
			cfg:          config.Config{Provider: "openai", Model: "gpt-5"},
			wantProvider: "openai", wantModel: "gpt-5",
		},
		{
			name:         "bare model defaults to anthropic (back-compat)",
			cfg:          config.Config{Model: "claude-opus-4-8"},
			wantProvider: "anthropic", wantModel: "claude-opus-4-8",
		},
		{
			name:         "known provider prefix is split",
			cfg:          config.Config{Model: "google/gemini-2.5-pro"},
			wantProvider: "google", wantModel: "gemini-2.5-pro",
		},
		{
			name:         "openrouter-style slug is left intact under explicit provider",
			cfg:          config.Config{Provider: "openrouter", Model: "anthropic/claude-opus-4-8"},
			wantProvider: "openrouter", wantModel: "anthropic/claude-opus-4-8",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, m := resolveModel(tt.cfg)
			if p != tt.wantProvider || m != tt.wantModel {
				t.Errorf("resolveModel() = (%q, %q), want (%q, %q)", p, m, tt.wantProvider, tt.wantModel)
			}
		})
	}
}

func TestResolveProviderInfoCustom(t *testing.T) {
	cfg := config.Config{
		Providers: map[string]config.ProviderConfig{
			"mygateway": {BaseURL: "https://gw.example/v1", APIKeyEnv: "GW_KEY"},
		},
	}
	info, err := resolveProviderInfo(cfg, "mygateway")
	if err != nil {
		t.Fatalf("resolveProviderInfo() error = %v", err)
	}
	if info.Protocol != ProtocolOpenAI {
		t.Errorf("custom provider protocol = %q, want openai", info.Protocol)
	}
	if info.BaseURL != "https://gw.example/v1" {
		t.Errorf("base url = %q", info.BaseURL)
	}
}

func TestResolveProviderInfoOverridesBuiltin(t *testing.T) {
	cfg := config.Config{
		Providers: map[string]config.ProviderConfig{
			"ollama": {BaseURL: "http://10.0.0.5:11434/v1"},
		},
	}
	info, err := resolveProviderInfo(cfg, "ollama")
	if err != nil {
		t.Fatal(err)
	}
	if info.BaseURL != "http://10.0.0.5:11434/v1" {
		t.Errorf("override base url = %q", info.BaseURL)
	}
	if !info.Local {
		t.Error("ollama should remain local after override")
	}
}

func TestResolveProviderInfoUnknown(t *testing.T) {
	if _, err := resolveProviderInfo(config.Config{}, "bogus"); err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestNewNoCredentialsIsSentinel(t *testing.T) {
	// Point config dir at an empty temp dir and clear the env so neither a stored
	// credential nor ANTHROPIC_API_KEY is available.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	t.Setenv("ANTHROPIC_API_KEY", "")

	_, err := New(config.Config{Provider: "anthropic", Model: "claude-opus-4-8"})
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("New() error = %v, want wrapped ErrNoCredentials", err)
	}
}

func TestNewLocalNeedsNoKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	p, err := New(config.Config{Provider: "ollama", Model: "llama3.1", MaxTokens: 4096})
	if err != nil {
		t.Fatalf("New(ollama) error = %v, want nil (local needs no key)", err)
	}
	if p.Provider() != "ollama" || p.Model() != "llama3.1" {
		t.Errorf("got provider %q model %q", p.Provider(), p.Model())
	}
}

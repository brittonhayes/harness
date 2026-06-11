package llm

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/brittonhayes/vala/internal/auth"
	"github.com/brittonhayes/vala/internal/config"
)

// ErrNoCredentials marks the "not connected yet" condition: a non-local provider
// with no API key. Callers use errors.Is to distinguish it (launch the REPL and
// let the operator run /connect) from a genuine configuration error.
var ErrNoCredentials = errors.New("no provider credentials")

// NoCredentialsError is the concrete error returned when a provider has no key.
// It renders a friendly, actionable message and satisfies errors.Is for
// ErrNoCredentials.
type NoCredentialsError struct {
	Provider string
	EnvVar   string
}

func (e *NoCredentialsError) Error() string {
	msg := fmt.Sprintf("no credentials for provider %q: run `vala connect` to set one up", e.Provider)
	if e.EnvVar != "" {
		msg += fmt.Sprintf(" (or set %s)", e.EnvVar)
	}
	return msg
}

func (e *NoCredentialsError) Is(target error) bool { return target == ErrNoCredentials }

// New constructs the active Provider from configuration. It resolves the
// provider id and model, looks up the endpoint and protocol (built-in or
// custom), and finds an API key from the environment first, then the stored
// credential. It returns a friendly error — pointing at `vala connect` — when a
// non-local provider has no key, so the very first run guides the operator
// instead of failing cryptically.
func New(cfg config.Config) (Provider, error) {
	providerID, model := resolveModel(cfg)

	info, err := resolveProviderInfo(cfg, providerID)
	if err != nil {
		return nil, err
	}
	if model == "" {
		model = info.DefaultModel
	}

	store, err := auth.Load()
	if err != nil {
		return nil, fmt.Errorf("read credentials: %w", err)
	}
	cred, _ := store.Get(providerID)

	// Endpoint precedence: stored credential override → config override →
	// registry default.
	baseURL := info.BaseURL
	if cred.BaseURL != "" {
		baseURL = cred.BaseURL
	}

	// Key precedence: environment variable → stored credential.
	apiKey := ""
	if info.APIKeyEnv != "" {
		apiKey = os.Getenv(info.APIKeyEnv)
	}
	if apiKey == "" {
		apiKey = cred.Key
	}
	if apiKey == "" && !info.Local {
		return nil, &NoCredentialsError{Provider: providerID, EnvVar: info.APIKeyEnv}
	}

	contextWindow := contextWindowFor(providerID, model)

	switch info.Protocol {
	case ProtocolAnthropic:
		return newAnthropic(apiKey, baseURL, model, cfg.MaxTokens, contextWindow), nil
	case ProtocolOpenAI:
		return newOpenAI(info, apiKey, baseURL, model, cfg.MaxTokens, contextWindow), nil
	default:
		return nil, fmt.Errorf("provider %q has unknown protocol %q", providerID, info.Protocol)
	}
}

// resolveModel determines the active provider id and model from config. The
// provider field is authoritative; absent it, a "provider/model" prefix is
// honored only when the prefix is a known built-in (so OpenRouter model ids,
// which themselves contain a slash, are left intact); otherwise vala defaults to
// Anthropic, preserving configs that predate multi-provider support.
func resolveModel(cfg config.Config) (provider, model string) {
	if cfg.Provider != "" {
		return cfg.Provider, cfg.Model
	}
	if before, after, found := strings.Cut(cfg.Model, "/"); found {
		if _, ok := builtins[before]; ok {
			return before, after
		}
	}
	return "anthropic", cfg.Model
}

// resolveProviderInfo returns the endpoint/protocol/auth description for a
// provider id. It starts from the built-in registry and overlays any matching
// entry in the project's `providers` config; a provider present only in config
// (a custom OpenAI-compatible gateway) is built from that entry alone.
func resolveProviderInfo(cfg config.Config, id string) (ProviderInfo, error) {
	info, isBuiltin := builtins[id]
	override, hasOverride := cfg.Providers[id]

	if !isBuiltin && !hasOverride {
		return ProviderInfo{}, fmt.Errorf("unknown provider %q: choose a built-in (run `vala connect`) or define it under `providers` in .vala.json", id)
	}

	if !isBuiltin {
		// Custom provider defined entirely in config. Default to the
		// OpenAI-compatible protocol since that covers the long tail.
		info = ProviderInfo{ID: id, Name: id, Protocol: ProtocolOpenAI}
	}
	if hasOverride {
		if override.BaseURL != "" {
			info.BaseURL = override.BaseURL
		}
		if override.Protocol != "" {
			info.Protocol = Protocol(override.Protocol)
		}
		if override.APIKeyEnv != "" {
			info.APIKeyEnv = override.APIKeyEnv
		}
		if override.Model != "" {
			info.DefaultModel = override.Model
		}
		if override.Local {
			info.Local = true
		}
	}
	if info.Protocol == ProtocolOpenAI && info.BaseURL == "" {
		return ProviderInfo{}, fmt.Errorf("provider %q needs a base_url (set it under `providers` in .vala.json)", id)
	}
	return info, nil
}

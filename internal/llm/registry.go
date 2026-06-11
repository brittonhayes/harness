package llm

import "sort"

// Protocol names the wire format a provider speaks. vala ships two, which
// between them cover every provider it supports: Anthropic's Messages API and
// the OpenAI Chat Completions API. The latter is spoken (with only a different
// base URL) by OpenAI, Google's OpenAI-compatible endpoint, OpenRouter, Groq,
// DeepSeek, xAI, and local runtimes like Ollama and LM Studio.
type Protocol string

const (
	ProtocolAnthropic Protocol = "anthropic"
	ProtocolOpenAI    Protocol = "openai"
)

// ProviderInfo is the static description of a known provider: which protocol it
// speaks, where it lives, how to authenticate, and a sensible default model.
// It is the single place a new provider is registered.
type ProviderInfo struct {
	// ID is the short identifier used in config and the connect flow.
	ID string
	// Name is the human label shown in the connect picker.
	Name string
	// Protocol is the wire format the provider speaks.
	Protocol Protocol
	// BaseURL is the API endpoint. Empty means the protocol's own default
	// (Anthropic's SDK default for ProtocolAnthropic).
	BaseURL string
	// APIKeyEnv is the environment variable checked for an API key before the
	// stored credential. Empty for providers that need no key (local runtimes).
	APIKeyEnv string
	// DefaultModel is selected when config names the provider but no model.
	DefaultModel string
	// Local marks a provider that runs on the operator's machine and needs no
	// API key (Ollama, LM Studio). The connect flow asks for a base URL instead.
	Local bool
	// OAuth marks a provider that supports a browser-based subscription login
	// (e.g. Claude Pro/Max), letting an operator connect without a raw API key.
	// The connect flow offers it as an alternative to pasting a key.
	OAuth bool
}

// builtins is vala's registry of known providers. The OpenAI-compatible entries
// differ only by base URL and default model — the protocol code is shared.
var builtins = map[string]ProviderInfo{
	"anthropic": {
		ID: "anthropic", Name: "Anthropic (Claude)",
		Protocol: ProtocolAnthropic, APIKeyEnv: "ANTHROPIC_API_KEY",
		DefaultModel: "claude-opus-4-8", OAuth: true,
	},
	"openai": {
		ID: "openai", Name: "OpenAI (ChatGPT)",
		Protocol: ProtocolOpenAI, BaseURL: "https://api.openai.com/v1",
		APIKeyEnv: "OPENAI_API_KEY", DefaultModel: "gpt-5",
	},
	"google": {
		ID: "google", Name: "Google (Gemini)",
		Protocol:  ProtocolOpenAI,
		BaseURL:   "https://generativelanguage.googleapis.com/v1beta/openai",
		APIKeyEnv: "GEMINI_API_KEY", DefaultModel: "gemini-2.5-pro",
	},
	"openrouter": {
		ID: "openrouter", Name: "OpenRouter (any model)",
		Protocol: ProtocolOpenAI, BaseURL: "https://openrouter.ai/api/v1",
		APIKeyEnv: "OPENROUTER_API_KEY", DefaultModel: "anthropic/claude-opus-4-8",
	},
	"groq": {
		ID: "groq", Name: "Groq (fast inference)",
		Protocol: ProtocolOpenAI, BaseURL: "https://api.groq.com/openai/v1",
		APIKeyEnv: "GROQ_API_KEY", DefaultModel: "llama-3.3-70b-versatile",
	},
	"deepseek": {
		ID: "deepseek", Name: "DeepSeek",
		Protocol: ProtocolOpenAI, BaseURL: "https://api.deepseek.com/v1",
		APIKeyEnv: "DEEPSEEK_API_KEY", DefaultModel: "deepseek-chat",
	},
	"xai": {
		ID: "xai", Name: "xAI (Grok)",
		Protocol: ProtocolOpenAI, BaseURL: "https://api.x.ai/v1",
		APIKeyEnv: "XAI_API_KEY", DefaultModel: "grok-4",
	},
	"ollama": {
		ID: "ollama", Name: "Ollama (local)",
		Protocol: ProtocolOpenAI, BaseURL: "http://localhost:11434/v1",
		Local: true, DefaultModel: "llama3.1",
	},
	"lmstudio": {
		ID: "lmstudio", Name: "LM Studio (local)",
		Protocol: ProtocolOpenAI, BaseURL: "http://localhost:1234/v1",
		Local: true, DefaultModel: "local-model",
	},
}

// Builtin returns the registered provider with the given id, if known.
func Builtin(id string) (ProviderInfo, bool) {
	info, ok := builtins[id]
	return info, ok
}

// Providers returns every known provider id, sorted, with recommended providers
// first so the connect picker leads with the most common choices.
func Providers() []ProviderInfo {
	// Recommendation order mirrors what most operators reach for first.
	priority := map[string]int{
		"anthropic": 0, "openai": 1, "google": 2,
		"openrouter": 3, "ollama": 4, "lmstudio": 5,
	}
	ids := make([]string, 0, len(builtins))
	for id := range builtins {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		pi, oki := priority[ids[i]]
		pj, okj := priority[ids[j]]
		if oki != okj {
			return oki // prioritized ids sort before unprioritized
		}
		if oki && okj && pi != pj {
			return pi < pj
		}
		return ids[i] < ids[j]
	})
	out := make([]ProviderInfo, 0, len(ids))
	for _, id := range ids {
		out = append(out, builtins[id])
	}
	return out
}

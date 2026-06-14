package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/brittonhayes/vala/internal/auth"
	"github.com/brittonhayes/vala/internal/config"
	"github.com/brittonhayes/vala/internal/llm"
	"github.com/brittonhayes/vala/internal/ui/setup"
)

// providerConfigured reports whether a model provider credential is available —
// from the environment, a stored credential, or a local runtime that needs no
// key. It mirrors the resolution llm.New performs, without constructing a client.
func providerConfigured(cfg config.Config) bool {
	id := cfg.Provider
	if id == "" {
		id = "anthropic"
	}
	if info, ok := llm.Builtin(id); ok {
		if info.Local {
			return true // local servers need no key; the default URL works
		}
		if info.APIKeyEnv != "" && os.Getenv(info.APIKeyEnv) != "" {
			return true
		}
	}
	if pc, ok := cfg.Providers[id]; ok {
		if pc.Local {
			return true
		}
		if pc.APIKeyEnv != "" && os.Getenv(pc.APIKeyEnv) != "" {
			return true
		}
	}
	if cfg.APIKey != "" {
		return true
	}
	if store, err := auth.Load(); err == nil {
		if _, ok := store.Get(id); ok {
			return true
		}
	}
	return false
}

// evidenceConfigured reports whether any MCP evidence source is configured —
// without it the agent has nothing to hunt in. The reserved Notion MCP server is
// part of the brain, not evidence, so it does not satisfy this predicate.
func evidenceConfigured(cfg config.Config) bool {
	for _, s := range cfg.MCP {
		if !cfg.IsNotionSearchServer(s) {
			return true
		}
	}
	return false
}

// mcpNames lists the configured evidence-source names for the setup hub.
func mcpNames(cfg config.Config) []string {
	names := make([]string, 0, len(cfg.MCP))
	for _, s := range cfg.MCP {
		if cfg.IsNotionSearchServer(s) {
			continue
		}
		names = append(names, s.Name)
	}
	return names
}

// notionSearchConfigured reports whether the Notion brain's MCP search backend
// is configured. It is stored as the reserved MCP server named "notion".
func notionSearchConfigured(cfg config.Config) bool {
	_, ok := cfg.NotionSearchServer()
	return ok
}

// brainSummary describes the configured brain backend for the setup hub.
func brainSummary(cfg config.Config) string {
	switch {
	case brainConfigured(cfg) && !brainComplete(cfg):
		return "Notion — needs repair"
	case brainConfigured(cfg) && !notionSearchConfigured(cfg):
		return "Notion — needs search MCP"
	case brainConfigured(cfg):
		return "Notion (shared)"
	case cfg.BrainFile != "":
		return "on-disk"
	default:
		return ""
	}
}

// brainReady reports whether the brain is set up AND usable: a Notion brain must
// have every store's data source and the Notion MCP search backend; an on-disk
// brain just needs a path. An incomplete Notion brain is configured but not
// ready — it routes to repair/search setup rather than counting as done.
func brainReady(cfg config.Config) bool {
	if brainConfigured(cfg) {
		return brainComplete(cfg) && notionSearchConfigured(cfg)
	}
	return cfg.BrainFile != ""
}

// setupComplete reports whether all three surfaces are ready.
func setupComplete(cfg config.Config) bool {
	return providerConfigured(cfg) && brainReady(cfg) && evidenceConfigured(cfg)
}

// maybeRunSetup launches the onboarding wizard when the interactive session
// detects an unconfigured surface (provider, brain, or evidence). It returns
// proceed=false only when the operator aborts vala from the wizard. A completed
// or skipped wizard records a dismissal so the session is not gated again; the
// operator re-runs it on demand with `vala setup`. force shows every step even
// when already configured.
func maybeRunSetup(ctx context.Context, cfg config.Config, cwd string, force bool) (proceed bool, err error) {
	// A configured-but-incomplete Notion brain is a broken state (failing API
	// calls against a missing data source). A Notion brain without its MCP search
	// backend is also incomplete for recall quality. We open the wizard even when
	// the first-run prompt was dismissed, so the operator is never stuck.
	brokenBrain := brainConfigured(cfg) && !brainComplete(cfg)
	brainNeedsSearch := brainConfigured(cfg) && brainComplete(cfg) && !notionSearchConfigured(cfg)
	if !force {
		if flagRequireBrain && !(brainConfigured(cfg) || cfg.BrainFile != "") {
			return false, fmt.Errorf("no brain is configured; run `vala setup` (or unset --require-brain)")
		}
		if !brokenBrain && !brainNeedsSearch && (setupComplete(cfg) || flagNoInitPrompt || initPromptDismissed(cwd)) {
			return true, nil
		}
	}

	res, err := setup.Run(ctx, setup.Options{
		Cwd:              cwd,
		ProviderOK:       providerConfigured(cfg),
		BrainOK:          brainReady(cfg),
		Model:            cfg.Provider + " · " + cfg.Model,
		Brain:            brainSummary(cfg),
		BrainSearchOK:    notionSearchConfigured(cfg),
		Evidence:         mcpNames(cfg),
		Force:            force,
		Notion:           cfg.Notion,
		BrainNeedsRepair: brokenBrain || brainNeedsSearch,
	})
	if err != nil {
		return false, err
	}
	if res.Quit {
		return false, nil
	}
	// Provision the on-disk brain the operator chose, reusing the helper that also
	// scaffolds VALA.md and validates the file opens. A provisioned/repaired Notion
	// brain saves its own IDs in the wizard; here we just scaffold the operator
	// context so both brain choices land in the same place.
	if res.BrainLocal {
		if err := provisionLocalBrain(cwd, ""); err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not set up local brain:", err)
		}
	}
	if res.BrainNotion {
		scaffoldOperatorContext(cwd)
	}
	if !force {
		dismissInitPrompt(cwd)
	}
	return true, nil
}

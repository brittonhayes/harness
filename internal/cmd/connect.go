package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/brittonhayes/vala/internal/auth"
	"github.com/brittonhayes/vala/internal/config"
	"github.com/brittonhayes/vala/internal/llm"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var connectCmd = &cobra.Command{
	Use:   "connect [provider]",
	Short: "Connect an LLM provider (Anthropic, OpenAI, Google, local, …)",
	Long: `Connect picks a model provider and stores its credential so vala works out of
the box. Run it with no arguments for a guided picker, or name a provider to
jump straight to it:

  vala connect              # choose from the list
  vala connect anthropic    # set up Anthropic (Claude)
  vala connect ollama       # point vala at a local Ollama server

API keys are saved to ~/.config/vala/auth.json (mode 0600), never to the
project config. The chosen provider and model are recorded in ./.vala.json so
the next session uses them. Environment variables (e.g. ANTHROPIC_API_KEY)
always take precedence and need no connect step.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		preselect := ""
		if len(args) == 1 {
			preselect = args[0]
		}
		return runConnect(preselect)
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)
}

// runConnect drives the interactive provider setup and persists the result.
func runConnect(preselect string) error {
	store, err := auth.Load()
	if err != nil {
		return fmt.Errorf("read credentials: %w", err)
	}

	info, err := pickProvider(store, preselect)
	if err != nil {
		return err
	}

	cred := auth.Credential{Type: "api"}
	switch {
	case info.Local:
		// Local runtimes need no key — just where the server lives.
		url := promptDefault(fmt.Sprintf("Base URL [%s]: ", info.BaseURL), info.BaseURL)
		cred.BaseURL = url
		fmt.Fprintf(os.Stderr, "✓ %s will use %s (no API key needed)\n", info.Name, url)
	default:
		key, err := readSecret(fmt.Sprintf("Enter your %s API key: ", info.Name))
		if err != nil {
			return err
		}
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("no API key entered")
		}
		cred.Key = key
	}

	// Model selection: default to the provider's recommended model, with the
	// curated catalog shown as a hint.
	if models := llm.CatalogModels(info.ID); len(models) > 0 {
		fmt.Fprintf(os.Stderr, "  known models: %s\n", strings.Join(models, ", "))
	}
	model := promptDefault(fmt.Sprintf("Model [%s]: ", info.DefaultModel), info.DefaultModel)
	cred.Model = model

	if err := store.Set(info.ID, cred); err != nil {
		return fmt.Errorf("save credential: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := config.SaveProvider(cwd, info.ID, model); err != nil {
		return fmt.Errorf("save provider to .vala.json: %w", err)
	}

	path, _ := auth.Path()
	fmt.Fprintf(os.Stderr, "\n✓ Connected %s (%s)\n", info.Name, model)
	fmt.Fprintf(os.Stderr, "  credential → %s\n", path)
	fmt.Fprintf(os.Stderr, "  provider   → ./.vala.json\n")
	fmt.Fprintf(os.Stderr, "  run `vala` to start a session.\n")
	return nil
}

// pickProvider resolves the provider to connect, either from a preselected id or
// by prompting the operator with a numbered list.
func pickProvider(store *auth.Store, preselect string) (llm.ProviderInfo, error) {
	if preselect != "" {
		info, ok := llm.Builtin(preselect)
		if !ok {
			return llm.ProviderInfo{}, fmt.Errorf("unknown provider %q; run `vala connect` to see the list", preselect)
		}
		return info, nil
	}

	providers := llm.Providers()
	fmt.Fprintln(os.Stderr, "Connect a provider:")
	for i, p := range providers {
		mark := " "
		if _, ok := store.Get(p.ID); ok {
			mark = "✓"
		} else if p.APIKeyEnv != "" && os.Getenv(p.APIKeyEnv) != "" {
			mark = "✓"
		}
		fmt.Fprintf(os.Stderr, "  %s %2d. %-26s %s\n", mark, i+1, p.Name, hintFor(p))
	}

	choice := promptDefault("\nSelect a provider [1]: ", "1")
	if n, err := strconv.Atoi(choice); err == nil {
		if n < 1 || n > len(providers) {
			return llm.ProviderInfo{}, fmt.Errorf("choice %d out of range", n)
		}
		return providers[n-1], nil
	}
	// Allow typing the provider id directly.
	if info, ok := llm.Builtin(strings.TrimSpace(choice)); ok {
		return info, nil
	}
	return llm.ProviderInfo{}, fmt.Errorf("unknown provider %q", choice)
}

// hintFor describes how a provider authenticates, shown in the picker.
func hintFor(p llm.ProviderInfo) string {
	if p.Local {
		return "local server, no key"
	}
	if p.APIKeyEnv != "" {
		return "API key (or " + p.APIKeyEnv + ")"
	}
	return "API key"
}

// promptDefault reads a line, returning def when the operator just presses enter.
func promptDefault(prompt, def string) string {
	fmt.Fprint(os.Stderr, prompt)
	line, err := readLine()
	if err != nil || strings.TrimSpace(line) == "" {
		return def
	}
	return strings.TrimSpace(line)
}

// readSecret reads a secret without echoing it when stdin is a terminal, falling
// back to a plain line read otherwise (pipes, CI).
func readSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Fprintln(os.Stderr)
		return strings.TrimSpace(string(b)), err
	}
	return readLine()
}

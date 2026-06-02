// Package cmd wires the harness CLI together with cobra.
package cmd

import (
	"fmt"
	"os"

	"github.com/brittonhayes/harness/internal/agent"
	"github.com/brittonhayes/harness/internal/config"
	"github.com/brittonhayes/harness/internal/llm"
	"github.com/brittonhayes/harness/internal/permission"
	"github.com/brittonhayes/harness/internal/session"
	"github.com/brittonhayes/harness/internal/tool"
	"github.com/brittonhayes/harness/internal/tools"
	"github.com/brittonhayes/harness/internal/ui"
	"github.com/spf13/cobra"
)

// persisted flag values shared across commands.
var (
	flagModel      string
	flagPermission string
)

// rootCmd starts the interactive REPL by default.
var rootCmd = &cobra.Command{
	Use:   "harness",
	Short: "Agentic security detection & response harness",
	Long: `Harness is an agentic harness for security detection & response work.

It drives an LLM agent that can investigate, author and validate Sigma
detection rules, run shell/file tools, and document findings in Notion via the
ntn CLI.

Run with no arguments to start an interactive session, or use "harness run"
for a single non-interactive task.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		built, err := build()
		if err != nil {
			return err
		}
		sess, err := session.New(session.DefaultDir())
		if err != nil {
			fmt.Fprintln(os.Stderr, "warning: transcript disabled:", err)
		}
		ag := agent.New(built.client, built.registry, built.gate, built.cwd, built.cfg.MaxSteps)
		repl := ui.New(ag, built.gate, sess, built.client.Model())
		return repl.Run(cmd.Context())
	},
}

// Execute is the CLI entry point.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagModel, "model", "", "Anthropic model ID (overrides config)")
	rootCmd.PersistentFlags().StringVar(&flagPermission, "permission", "", "permission mode: ask | allow | deny")
	rootCmd.AddCommand(runCmd, versionCmd)
}

// built bundles the constructed dependencies for a command.
type built struct {
	cfg      config.Config
	cwd      string
	client   *llm.Client
	registry *tool.Registry
	gate     *permission.Gate
}

// build resolves config + flags and constructs the shared dependencies.
func build() (*built, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(cwd)
	if err != nil {
		return nil, err
	}
	if flagModel != "" {
		cfg.Model = flagModel
	}
	if flagPermission != "" {
		cfg.Permission = flagPermission
	}

	client, err := llm.New(cfg)
	if err != nil {
		return nil, err
	}
	registry := tools.Default(cwd)
	gate := permission.New(permission.Parse(cfg.Permission), cfg.Allowlist)

	return &built{cfg: cfg, cwd: cwd, client: client, registry: registry, gate: gate}, nil
}

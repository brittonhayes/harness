package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brittonhayes/vala/internal/brain"
	"github.com/brittonhayes/vala/internal/config"
)

// defaultBrainFile is where a local file-backed brain lives inside a project.
const defaultBrainFile = ".vala/brain.json"

// provisionLocalBrain sets up a durable on-disk brain with no Notion account: it
// records the brain-file path in .vala.json and scaffolds a starter VALA.md. The
// JSON file itself is created lazily on the first hunt. This is the
// zero-dependency path to a persistent brain, used by the setup wizard's on-disk
// choice.
func provisionLocalBrain(cwd, brainFile string) error {
	if brainFile == "" {
		brainFile = defaultBrainFile
	}
	if err := config.SaveBrainFile(cwd, brainFile); err != nil {
		return fmt.Errorf("write .vala.json: %w", err)
	}
	// Validate the path resolves to an openable brain before we report success.
	path := brainFile
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	if _, err := brain.NewFile(path); err != nil {
		return fmt.Errorf("prepare brain file: %w", err)
	}
	fmt.Fprintf(os.Stderr, "✓ Local brain configured at %s — hunts now persist across sessions\n", brainFile)
	scaffoldOperatorContext(cwd)
	fmt.Fprintln(os.Stderr, "  Next: run `vala` and try — \"queue a hunt: did anyone disable GuardDuty?\"")
	return nil
}

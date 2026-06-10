package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"strings"

	"github.com/brittonhayes/vala/internal/agent"
	"github.com/brittonhayes/vala/internal/tool"
)

//go:embed remember.md
var rememberDescription string

// rememberSection is the heading under which the remember tool collects durable
// operator facts in VALA.md.
const rememberSection = "## Remembered"

// Remember appends a durable environment fact to the project's VALA.md so it
// primes every future session. It is the write counterpart to the operator
// context vala loads at startup: the agent can grow its own standing memory as a
// hunt teaches it how the environment really works. Not read-only;
// permission-gated.
type Remember struct{ Dir string }

func (r *Remember) Name() string        { return "remember" }
func (r *Remember) Description() string { return rememberDescription }
func (r *Remember) ReadOnly() bool      { return false }

func (r *Remember) Schema() tool.Schema {
	return tool.Schema{
		Properties: map[string]any{
			"fact": map[string]any{
				"type":        "string",
				"description": "A durable fact about this environment worth recalling in every future session — a log-source location, a known-good baseline, a naming convention, a crown-jewel system. One specific sentence; never a secret.",
			},
		},
		Required: []string{"fact"},
	}
}

type rememberInput struct {
	Fact string `json:"fact"`
}

func (r *Remember) Run(_ context.Context, input json.RawMessage) (tool.Result, error) {
	var in rememberInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Errorf("invalid input: %v", err), nil
	}
	fact := strings.TrimSpace(in.Fact)
	if fact == "" {
		return tool.Errorf("fact is required"), nil
	}

	path := resolve(r.Dir, agent.OperatorContextFile)
	existing := ""
	if data, err := os.ReadFile(path); err == nil {
		existing = string(data)
	} else if !os.IsNotExist(err) {
		return tool.Errorf("cannot read %s: %v", agent.OperatorContextFile, err), nil
	}

	if err := os.WriteFile(path, []byte(appendFact(existing, fact)), 0o644); err != nil {
		return tool.Errorf("cannot write %s: %v", agent.OperatorContextFile, err), nil
	}
	return tool.Text("remembered in " + agent.OperatorContextFile + ": " + fact), nil
}

// appendFact returns content with fact added as a bullet under the Remembered
// section. It creates that section (and a heading for a brand-new file) when
// absent, and otherwise inserts the bullet at the end of the existing section so
// related facts stay together. The result always ends with a newline.
func appendFact(content, fact string) string {
	bullet := "- " + fact

	if strings.TrimSpace(content) == "" {
		return "# vala operator context\n\n" + rememberSection + "\n\n" + bullet + "\n"
	}

	lines := strings.Split(content, "\n")
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == rememberSection {
			start = i
			break
		}
	}
	if start == -1 {
		out := strings.TrimRight(content, "\n") + "\n\n" + rememberSection + "\n\n" + bullet + "\n"
		return out
	}

	// Insert at the end of the Remembered section: just before the next heading
	// (or EOF), backing up over trailing blank lines so the bullet sits with its
	// peers rather than after a gap.
	insert := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "# ") || strings.HasPrefix(lines[i], "## ") {
			insert = i
			break
		}
	}
	for insert > start+1 && strings.TrimSpace(lines[insert-1]) == "" {
		insert--
	}

	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:insert]...)
	out = append(out, bullet)
	out = append(out, lines[insert:]...)
	joined := strings.Join(out, "\n")
	if !strings.HasSuffix(joined, "\n") {
		joined += "\n"
	}
	return joined
}

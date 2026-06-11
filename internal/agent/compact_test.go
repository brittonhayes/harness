package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/brittonhayes/vala/internal/llm"
)

// TestCompactEmptyHistory verifies the empty-history short-circuit: it must be a
// true no-op that never touches the LLM client (here nil, so a call would panic).
func TestCompactEmptyHistory(t *testing.T) {
	a := &Agent{} // llm is nil; the short-circuit must avoid using it
	hist, summary, err := a.Compact(context.Background(), nil, "")
	if err != nil {
		t.Fatalf("Compact(empty) error = %v, want nil", err)
	}
	if summary != "" {
		t.Fatalf("Compact(empty) summary = %q, want empty", summary)
	}
	if len(hist) != 0 {
		t.Fatalf("Compact(empty) history len = %d, want 0", len(hist))
	}
}

func TestCompactionSystemPromptSections(t *testing.T) {
	prompt := compactionSystemPrompt()
	sections := []string{
		"Primary Request and Intent",
		"Key Technical Concepts",
		"Files and Resources",
		"Errors and Fixes",
		"Problem Solving",
		"Pending Tasks",
		"Current Work",
		"Next Step",
	}
	for _, s := range sections {
		if !strings.Contains(prompt, s) {
			t.Errorf("compactionSystemPrompt missing section %q", s)
		}
	}
}

func TestBuildContinuationHistory(t *testing.T) {
	summary := "## Primary Request and Intent\nDo the thing."
	hist := buildContinuationHistory(summary)
	if len(hist) != 1 {
		t.Fatalf("buildContinuationHistory len = %d, want 1", len(hist))
	}
	if hist[0].Role != llm.RoleUser {
		t.Fatalf("seed role = %q, want user", hist[0].Role)
	}
	// The single text block must carry both the preamble and the summary.
	var got string
	for _, block := range hist[0].Content {
		if block.Type == llm.BlockText {
			got += block.Text
		}
	}
	if !strings.Contains(got, continuationPreamble) {
		t.Errorf("continuation history missing preamble")
	}
	if !strings.Contains(got, summary) {
		t.Errorf("continuation history missing summary")
	}
}

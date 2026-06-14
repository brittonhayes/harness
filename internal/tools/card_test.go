package tools

import (
	"fmt"
	"strings"
	"testing"

	"github.com/brittonhayes/vala/internal/brain"
	"github.com/brittonhayes/vala/internal/tool"
)

func TestUpdateCoverageCardSuggestsForThinCoverage(t *testing.T) {
	rc, _, _ := newHuntRC(t)
	res := run(t, &UpdateCoverage{RC: rc}, map[string]any{
		"technique":  "attack.t1562.001",
		"status":     brain.CoverageThin,
		"fidelity":   "low",
		"detections": "manual review only",
	})
	if res.IsError {
		t.Fatalf("update_coverage failed: %s", res.Content)
	}
	assertCard(t, res, "Coverage updated", "coverage", "attack.t1562.001 is Thin", "manual review only", "Consider queueing next")
}

func assertCard(t *testing.T, res tool.Result, title string, wants ...string) {
	t.Helper()
	if res.Card == nil {
		t.Fatalf("expected card on result %q", res.Content)
	}
	if res.Card.Title != title {
		t.Fatalf("card title = %q, want %q; card=%#v", res.Card.Title, title, res.Card)
	}
	text := cardText(res)
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("card missing %q:\n%s", want, text)
		}
	}
}

func cardText(res tool.Result) string {
	if res.Card == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n%s\n%s\n%s\n", res.Card.Kind, res.Card.Title, res.Card.Summary, res.Card.Link)
	for _, f := range res.Card.Fields {
		fmt.Fprintf(&b, "%s=%s\n", f.Label, f.Value)
	}
	for _, c := range res.Card.Changes {
		fmt.Fprintf(&b, "%s:%s->%s\n", c.Label, c.Before, c.After)
	}
	for _, s := range res.Card.Suggestions {
		fmt.Fprintf(&b, "%s %s %s %s %s %s %s\n", s.Title, s.Trigger, s.Hypothesis, s.Behavior, s.DataSource, s.Priority, s.MITRE)
	}
	if len(res.Card.Suggestions) > 0 {
		b.WriteString("Consider queueing next\n")
	}
	return b.String()
}

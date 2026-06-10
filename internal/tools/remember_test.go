package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brittonhayes/vala/internal/agent"
)

// TestRememberCreatesAndAppends covers the compounding path: the first fact
// creates VALA.md with a Remembered section, and a second fact joins it under
// the same heading rather than starting a new one.
func TestRememberCreatesAndAppends(t *testing.T) {
	dir := t.TempDir()
	tl := &Remember{Dir: dir}
	ctx := context.Background()

	if res, err := tl.Run(ctx, []byte(`{"fact":"auth logs live in Okta"}`)); err != nil || res.IsError {
		t.Fatalf("first remember failed: %v %q", err, res.Content)
	}
	if res, err := tl.Run(ctx, []byte(`{"fact":"svc-deploy rotates keys nightly"}`)); err != nil || res.IsError {
		t.Fatalf("second remember failed: %v %q", err, res.Content)
	}

	data, err := os.ReadFile(filepath.Join(dir, agent.OperatorContextFile))
	if err != nil {
		t.Fatalf("read VALA.md: %v", err)
	}
	got := string(data)
	if strings.Count(got, rememberSection) != 1 {
		t.Fatalf("want exactly one Remembered section, got:\n%s", got)
	}
	if !strings.Contains(got, "- auth logs live in Okta") || !strings.Contains(got, "- svc-deploy rotates keys nightly") {
		t.Fatalf("both facts should be present:\n%s", got)
	}
}

// TestRememberPreservesExistingSections asserts a fact lands under the
// Remembered heading without disturbing operator-authored sections.
func TestRememberPreservesExistingSections(t *testing.T) {
	dir := t.TempDir()
	seed := "# vala operator context\n\n## Environment\nProd AWS org.\n"
	if err := os.WriteFile(filepath.Join(dir, agent.OperatorContextFile), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	tl := &Remember{Dir: dir}
	if res, err := tl.Run(context.Background(), []byte(`{"fact":"CloudTrail is multi-region"}`)); err != nil || res.IsError {
		t.Fatalf("remember failed: %v %q", err, res.Content)
	}

	data, _ := os.ReadFile(filepath.Join(dir, agent.OperatorContextFile))
	got := string(data)
	if !strings.Contains(got, "## Environment\nProd AWS org.") {
		t.Fatalf("existing section was disturbed:\n%s", got)
	}
	if !strings.Contains(got, rememberSection+"\n\n- CloudTrail is multi-region") {
		t.Fatalf("fact not placed under Remembered section:\n%s", got)
	}
}

func TestRememberRejectsEmptyFact(t *testing.T) {
	tl := &Remember{Dir: t.TempDir()}
	res, err := tl.Run(context.Background(), []byte(`{"fact":"  "}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("empty fact should be rejected")
	}
}

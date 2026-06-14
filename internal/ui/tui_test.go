package ui

import (
	"strings"
	"testing"

	"github.com/brittonhayes/vala/internal/permission"
	"github.com/brittonhayes/vala/internal/session"
	"github.com/brittonhayes/vala/internal/tool"
)

// newTestModel builds a chatModel wired to a real session and gate but no agent,
// then sizes it as if the terminal reported 80x24. Tests that avoid submit-while-
// idle never touch the (nil) agent.
func newTestModel(t *testing.T) chatModel {
	t.Helper()
	sess, err := session.New(t.TempDir())
	if err != nil {
		t.Fatalf("session.New: %v", err)
	}
	r := &REPL{
		Gate:    permission.New(permission.ModeAsk, nil),
		Session: sess,
		Model:   "test-model",
		styles:  DefaultStyles(),
	}
	m := newChatModel(r)
	mm, _ := m.resize(80, 24)
	return mm.(chatModel)
}

// TestQueueWhileRunning verifies that submitting a message during an active turn
// enqueues it instead of starting a second turn (which would hit the nil agent).
func TestQueueWhileRunning(t *testing.T) {
	m := newTestModel(t)
	m.running = true
	m.ta.SetValue("look at the auth logs")

	res, _ := m.submit()
	m = res.(chatModel)

	if len(m.queue) != 1 || m.queue[0] != "look at the auth logs" {
		t.Fatalf("expected message queued, got queue=%v", m.queue)
	}
	if m.ta.Value() != "" {
		t.Fatalf("expected input cleared after submit, got %q", m.ta.Value())
	}
}

// TestPermissionApprove checks that answering a pending permission request
// unblocks the waiting agent goroutine with the right verdict.
func TestPermissionApprove(t *testing.T) {
	m := newTestModel(t)
	reply := make(chan bool, 1)
	m.perm = &permMsg{name: "bash", summary: "ls", reply: reply}

	m.answerPerm(true, false)

	select {
	case got := <-reply:
		if !got {
			t.Fatal("expected approve=true")
		}
	default:
		t.Fatal("expected a reply on the channel")
	}
	if m.perm != nil {
		t.Fatal("expected perm cleared after answer")
	}
}

// TestPermissionAlwaysAllowlists confirms that "always" both approves and adds
// the tool to the session allowlist.
func TestPermissionAlwaysAllowlists(t *testing.T) {
	m := newTestModel(t)
	reply := make(chan bool, 1)
	m.perm = &permMsg{name: "edit", summary: "detections/x.yml", reply: reply}

	m.answerPerm(true, true)

	if got := <-reply; !got {
		t.Fatal("expected approve=true")
	}
	if m.repl.Gate.Allow("edit", "detections/x.yml", false) != true {
		t.Fatal("expected edit to be allowlisted after 'always'")
	}
}

// TestInterruptResolvesPendingPermission ensures cancelling a turn also releases
// a goroutine blocked on a permission reply, so it cannot deadlock.
func TestInterruptResolvesPendingPermission(t *testing.T) {
	m := newTestModel(t)
	reply := make(chan bool, 1)
	m.perm = &permMsg{name: "bash", summary: "rm -rf /", reply: reply}
	canceled := false
	m.cancel = func() { canceled = true }

	m.interrupt()

	if got := <-reply; got {
		t.Fatal("expected interrupt to deny the pending permission")
	}
	if !canceled {
		t.Fatal("expected interrupt to cancel the turn context")
	}
	if m.perm != nil {
		t.Fatal("expected perm cleared after interrupt")
	}
}

func TestRenderResultUsesRichCard(t *testing.T) {
	m := newTestModel(t)

	out := m.renderResult(tool.Result{Content: "model-facing text", Card: &tool.Card{
		Title:   "Finding recorded",
		Summary: "A cited evidence row was added.",
		Fields: []tool.Field{
			{Label: "claim", Value: "Okta impossible travel candidate observed"},
			{Label: "evidence ID", Value: "evidence_123"},
		},
		Changes: []tool.Change{{Label: "finding", After: "Okta impossible travel candidate observed"}},
		Suggestions: []tool.Suggestion{{
			Title:      "Close the visibility gap",
			Trigger:    "Visibility gap",
			Hypothesis: "Telemetry can be restored.",
			DataSource: "cloudtrail",
			Priority:   "high",
		}},
	}})

	for _, want := range []string{"Finding recorded", "finding added", "claim", "evidence_123", "queue next"} {
		if !strings.Contains(out, want) {
			t.Fatalf("rich card missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "model-facing text") {
		t.Fatalf("rich card should not render raw model content:\n%s", out)
	}
}

func TestRenderResultFallbackAndError(t *testing.T) {
	m := newTestModel(t)

	fallback := m.renderResult(tool.Text("first line\nsecond line"))
	if !strings.Contains(fallback, "first line") || !strings.Contains(fallback, "+1 lines") {
		t.Fatalf("fallback missing compact output:\n%s", fallback)
	}

	errOut := m.renderResult(tool.Result{
		Content: "failed loudly",
		IsError: true,
		Card:    &tool.Card{Title: "Should not render"},
	})
	if !strings.Contains(errOut, "failed loudly") || strings.Contains(errOut, "Should not render") {
		t.Fatalf("error should keep compact error styling and ignore cards:\n%s", errOut)
	}
}

func TestWideViewStaysFullWidthWithoutRail(t *testing.T) {
	m := newTestModel(t)
	res, _ := m.resize(120, 24)
	m = res.(chatModel)
	view := m.View()

	if strings.Contains(view, "Active Hunt") || strings.Contains(view, "detection workspace") {
		t.Fatalf("view rendered removed rail:\n%s", view)
	}
	if m.vp.Width != 120 {
		t.Fatalf("wide viewport width = %d, want full terminal width 120", m.vp.Width)
	}
}

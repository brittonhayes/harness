package ui

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
)

func TestSplitCommand(t *testing.T) {
	tests := []struct {
		in       string
		wantName string
		wantArgs string
	}{
		{"help", "help", ""},
		{"compact", "compact", ""},
		{"compact focus on the auth hunt", "compact", "focus on the auth hunt"},
		{"  clear  ", "clear", ""},
		{"compact\textra", "compact", "extra"},
	}
	for _, tt := range tests {
		name, args := splitCommand(tt.in)
		if name != tt.wantName || args != tt.wantArgs {
			t.Errorf("splitCommand(%q) = (%q, %q), want (%q, %q)", tt.in, name, args, tt.wantName, tt.wantArgs)
		}
	}
}

func TestDispatchSlashHandling(t *testing.T) {
	tests := []struct {
		in          string
		wantHandled bool
	}{
		{"hunt for root logins", false}, // not a slash command
		{"/help", true},
		{"/clear", true},
		{"/compact", true},
		{"/bogus", true}, // unknown command is still "handled" (reports an error)
	}
	for _, tt := range tests {
		m := newTestModel(t)
		_, _, handled := m.dispatchSlash(tt.in)
		if handled != tt.wantHandled {
			t.Errorf("dispatchSlash(%q) handled = %v, want %v", tt.in, handled, tt.wantHandled)
		}
	}
}

func TestClearResetsContext(t *testing.T) {
	m := newTestModel(t)
	m.history = []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("hi"))}
	m.lastInputTokens = 1234
	m.append("some transcript block")

	res, _ := m.cmdClear("")
	m = res.(chatModel)

	if len(m.history) != 0 {
		t.Errorf("history not cleared: len = %d", len(m.history))
	}
	if m.lastInputTokens != 0 {
		t.Errorf("lastInputTokens = %d, want 0", m.lastInputTokens)
	}
	// The banner survives plus the "context cleared" notice; the prior block is gone.
	if len(m.blocks) != 2 {
		t.Fatalf("blocks = %d, want 2 (banner + notice)", len(m.blocks))
	}
}

func TestClearBusyIsNoOp(t *testing.T) {
	m := newTestModel(t)
	m.running = true
	m.history = []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("hi"))}

	res, _ := m.cmdClear("")
	m = res.(chatModel)

	if len(m.history) != 1 {
		t.Errorf("history cleared while running; len = %d, want 1", len(m.history))
	}
}

func TestShouldAutoCompact(t *testing.T) {
	hist := []anthropic.MessageParam{anthropic.NewUserMessage(anthropic.NewTextBlock("hi"))}
	tests := []struct {
		name      string
		window    int64
		threshold float64
		tokens    int64
		history   []anthropic.MessageParam
		want      bool
	}{
		{"below threshold", 1000, 0.8, 700, hist, false},
		{"at threshold", 1000, 0.8, 800, hist, true},
		{"above threshold", 1000, 0.8, 900, hist, true},
		{"disabled window", 0, 0.8, 900, hist, false},
		{"disabled threshold", 1000, 0, 900, hist, false},
		{"empty history", 1000, 0.8, 900, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(t)
			m.repl.ContextWindow = tt.window
			m.repl.AutoCompactThreshold = tt.threshold
			m.lastInputTokens = tt.tokens
			m.history = tt.history
			if got := m.shouldAutoCompact(); got != tt.want {
				t.Errorf("shouldAutoCompact() = %v, want %v", got, tt.want)
			}
		})
	}
}

package permission

import "testing"

func TestAllow(t *testing.T) {
	tests := []struct {
		name     string
		mode     Mode
		allow    []string
		tool     string
		readOnly bool
		prompt   Prompter
		want     bool
	}{
		{name: "read-only bypasses prompts", mode: ModeAsk, tool: "read", readOnly: true, want: true},
		{name: "auto mode permits writes", mode: ModeAuto, tool: "bash", want: true},
		{name: "allowlisted tool permitted in ask mode", mode: ModeAsk, allow: []string{"ntn"}, tool: "ntn", want: true},
		{name: "ask mode without prompter fails closed", mode: ModeAsk, tool: "bash", want: false},
		{name: "ask mode consults prompter (yes)", mode: ModeAsk, tool: "bash", prompt: func(_, _ string) bool { return true }, want: true},
		{name: "ask mode consults prompter (no)", mode: ModeAsk, tool: "bash", prompt: func(_, _ string) bool { return false }, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New(tt.mode, tt.allow)
			g.Prompt = tt.prompt
			if got := g.Allow(tt.tool, "summary", tt.readOnly); got != tt.want {
				t.Fatalf("Allow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllowToolAddsToSession(t *testing.T) {
	g := New(ModeAsk, nil)
	if g.Allow("ntn", "", false) {
		t.Fatal("expected ntn to be denied before allowlisting")
	}
	g.AllowTool("ntn")
	if !g.Allow("ntn", "", false) {
		t.Fatal("expected ntn to be allowed after AllowTool")
	}
}

func TestParse(t *testing.T) {
	for in, want := range map[string]Mode{"auto": ModeAuto, "allow": ModeAuto, "ask": ModeAsk, "deny": ModeAsk, "garbage": ModeAsk} {
		if got := Parse(in); got != want {
			t.Errorf("Parse(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNextModeTogglesAskAuto(t *testing.T) {
	if got := NextMode(ModeAsk); got != ModeAuto {
		t.Errorf("NextMode(ask) = %q, want auto", got)
	}
	if got := NextMode(ModeAuto); got != ModeAsk {
		t.Errorf("NextMode(auto) = %q, want ask", got)
	}
}

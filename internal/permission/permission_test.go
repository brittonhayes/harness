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
		{name: "read-only always allowed", mode: ModeDeny, tool: "read", readOnly: true, want: true},
		{name: "allow mode permits writes", mode: ModeAllow, tool: "bash", want: true},
		{name: "deny mode blocks writes", mode: ModeDeny, tool: "bash", want: false},
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
	for in, want := range map[string]Mode{"allow": ModeAllow, "deny": ModeDeny, "ask": ModeAsk, "garbage": ModeAsk} {
		if got := Parse(in); got != want {
			t.Errorf("Parse(%q) = %q, want %q", in, got, want)
		}
	}
}

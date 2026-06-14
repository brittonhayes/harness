// Package permission controls the session's interactivity level. Detection &
// response work routinely touches production systems (Notion, AWS, shells), so
// ask mode keeps the operator in the loop before non-read-only tools run. Auto
// mode is the hands-off profile for trusted sessions: the agent assumes
// reasonable defaults and records backlog items, hunts, intel, and other
// artifacts without step-by-step approval.
package permission

// Mode controls the default disposition for non-read-only tool calls.
type Mode string

const (
	// ModeAsk prompts the operator for each non-read-only, non-allowlisted call.
	ModeAsk Mode = "ask"
	// ModeAuto auto-approves every call (use for trusted, unattended runs).
	ModeAuto Mode = "auto"
)

// Parse converts a string into a Mode, defaulting to ModeAsk.
func Parse(s string) Mode {
	switch Mode(s) {
	case ModeAuto, "allow": // "allow" is accepted for older configs.
		return ModeAuto
	default:
		return ModeAsk
	}
}

// NextMode toggles between the two interactivity profiles. It backs the
// interactive shift+tab toggle so an operator can switch between hands-on and
// hands-off operation without restarting the session.
func NextMode(m Mode) Mode {
	if m == ModeAuto {
		return ModeAsk
	}
	return ModeAuto
}

// Prompter asks the operator to approve a single call. It receives the tool
// name and a short human summary of the call and returns true to allow it.
type Prompter func(tool, summary string) bool

// Gate decides whether a tool call may proceed.
type Gate struct {
	Mode      Mode
	allowlist map[string]bool
	Prompt    Prompter
}

// New builds a Gate from a mode and a list of always-allowed tool names.
func New(mode Mode, allowlist []string) *Gate {
	set := make(map[string]bool, len(allowlist))
	for _, name := range allowlist {
		set[name] = true
	}
	return &Gate{Mode: mode, allowlist: set}
}

// Allow reports whether a call should run. Read-only calls always proceed.
// Otherwise the decision follows the mode, the allowlist, and finally the
// interactive prompter (if one is configured).
func (g *Gate) Allow(tool, summary string, readOnly bool) bool {
	if readOnly {
		return true
	}
	return g.approveByMode(tool, summary)
}

// approveByMode applies the mode/allowlist/prompter decision behind Allow.
func (g *Gate) approveByMode(tool, summary string) bool {
	if g == nil || g.Mode == ModeAuto {
		return true
	}
	if g.allowlist[tool] {
		return true
	}
	if g.Prompt != nil {
		return g.Prompt(tool, summary)
	}
	// No way to ask and not explicitly allowed: fail closed.
	return false
}

// CycleMode advances the gate to the next permission profile and returns it.
func (g *Gate) CycleMode() Mode {
	g.Mode = NextMode(g.Mode)
	return g.Mode
}

// AllowTool adds a tool to the allowlist for the remainder of the session.
func (g *Gate) AllowTool(name string) {
	if g.allowlist == nil {
		g.allowlist = map[string]bool{}
	}
	g.allowlist[name] = true
}

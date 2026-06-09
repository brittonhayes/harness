package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// slashCommand is one operator command invoked with a leading slash. Handlers
// mutate and return the model like any other Bubble Tea update.
type slashCommand struct {
	name    string // without the slash, e.g. "compact"
	desc    string // one-line help text
	handler func(m chatModel, args string) (tea.Model, tea.Cmd)
}

// commands is the ordered registry rendered by /help and dispatched by submit.
func (m chatModel) commands() []slashCommand {
	return []slashCommand{
		{"help", "list available commands", chatModel.cmdHelp},
		{"clear", "clear the conversation and transcript (keep the banner)", chatModel.cmdClear},
		{"compact", "summarize the conversation to reclaim context; optional focus text", chatModel.cmdCompact},
	}
}

// dispatchSlash parses input (which begins with '/') and runs the matching
// command. The bool reports whether input was handled as a slash command; when
// false, the caller treats input as a normal turn. Any '/'-prefixed input is
// handled here — an unknown command reports an error rather than reaching the
// agent.
func (m chatModel) dispatchSlash(input string) (tea.Model, tea.Cmd, bool) {
	if !strings.HasPrefix(input, "/") {
		return m, nil, false
	}
	name, args := splitCommand(input[1:])
	for _, c := range m.commands() {
		if c.name == name {
			model, cmd := c.handler(m, args)
			return model, cmd, true
		}
	}
	m.append("  " + m.styles.Error.Render("unknown command: /"+name) + "  " + m.styles.Hint.Render("try /help"))
	return m, nil, true
}

// splitCommand separates the first whitespace-delimited token (the command name)
// from the remaining arguments.
func splitCommand(s string) (name, args string) {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i], strings.TrimSpace(s[i+1:])
	}
	return s, ""
}

func (m chatModel) cmdHelp(_ string) (tea.Model, tea.Cmd) {
	var b strings.Builder
	b.WriteString("  " + m.styles.BannerTag.Render("commands") + "\n")
	for _, c := range m.commands() {
		b.WriteString("  " + m.styles.ToolCall.Render("/"+c.name) + "  " + m.styles.Hint.Render(c.desc) + "\n")
	}
	m.append(strings.TrimRight(b.String(), "\n"))
	return m, nil
}

func (m chatModel) cmdClear(_ string) (tea.Model, tea.Cmd) {
	if m.running {
		m.append("  " + m.styles.Error.Render("busy") + "  " + m.styles.Hint.Render("wait for the current turn before clearing"))
		return m, nil
	}
	m.history = nil
	m.blocks = []string{m.banner()} // keep only the banner
	m.lastInputTokens = 0
	m.refreshViewport()
	m.append("  " + m.styles.Hint.Render("context cleared"))
	return m, nil
}

func (m chatModel) cmdCompact(args string) (tea.Model, tea.Cmd) {
	if m.running {
		m.append("  " + m.styles.Error.Render("busy") + "  " + m.styles.Hint.Render("wait for the current turn before compacting"))
		return m, nil
	}
	if len(m.history) == 0 {
		m.append("  " + m.styles.Hint.Render("nothing to compact yet"))
		return m, nil
	}
	return m.startCompaction(args, false)
}

package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/brittonhayes/vala/internal/auth"
	"github.com/brittonhayes/vala/internal/llm"
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
		{"connect", "choose or switch the LLM provider; /connect <provider> [key]", chatModel.cmdConnect},
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

// cmdConnect chooses or switches the active LLM provider mid-session. With no
// arguments it lists providers and their connection state; with a provider id it
// switches to that provider live, optionally storing an API key (remote) or base
// URL (local) passed inline. The credential is persisted to ~/.config/vala so it
// survives restarts; secrets typed inline are visible in the transcript, so the
// guided `vala connect` is the better path for first-time key entry.
func (m chatModel) cmdConnect(args string) (tea.Model, tea.Cmd) {
	if m.running {
		m.append("  " + m.styles.Error.Render("busy") + "  " + m.styles.Hint.Render("wait for the current turn before switching providers"))
		return m, nil
	}
	fields := strings.Fields(args)
	if len(fields) == 0 {
		m.append(m.connectList())
		return m, nil
	}

	id := fields[0]
	info, ok := llm.Builtin(id)
	if !ok {
		m.append("  " + m.styles.Error.Render("unknown provider: "+id) + "  " + m.styles.Hint.Render("run /connect to list providers"))
		return m, nil
	}

	// An inline secret stores a credential before switching: an API key for a
	// remote provider, or a base URL for a local server.
	if len(fields) >= 2 {
		if err := storeInlineCredential(info, fields[1]); err != nil {
			m.append("  " + m.styles.Error.Render("could not save credential: "+err.Error()))
			return m, nil
		}
	}

	if m.repl.Connect == nil {
		m.append("  " + m.styles.Error.Render("live connect unavailable in this session"))
		return m, nil
	}

	model := info.DefaultModel
	if store, err := auth.Load(); err == nil {
		if cred, ok := store.Get(id); ok && cred.Model != "" {
			model = cred.Model
		}
	}

	provider, err := m.repl.Connect(id, model)
	if err != nil {
		hint := "run `vala connect` for guided setup"
		if info.APIKeyEnv != "" {
			hint = "add a key: /connect " + id + " <api-key>  ·  or set " + info.APIKeyEnv
		} else if info.Local {
			hint = "point at your server: /connect " + id + " <base-url>"
		}
		m.append("  " + m.styles.Error.Render("not connected: "+err.Error()) + "\n  " + m.styles.Hint.Render(hint))
		return m, nil
	}

	m.repl.Agent.SetProvider(provider)
	m.repl.Model = provider.Provider() + " · " + provider.Model()
	m.append("  " + m.styles.BannerTag.Render("connected") + "  " +
		m.styles.ToolCall.Render(provider.Provider()) + "  " + m.styles.Hint.Render(provider.Model()))
	return m, nil
}

// connectList renders the provider picker shown by a bare /connect.
func (m chatModel) connectList() string {
	store, _ := auth.Load()
	var b strings.Builder
	b.WriteString("  " + m.styles.BannerTag.Render("providers") + "\n")
	for _, p := range llm.Providers() {
		mark := " "
		if store != nil {
			if _, ok := store.Get(p.ID); ok {
				mark = "✓"
			}
		}
		if mark == " " && p.APIKeyEnv != "" && os.Getenv(p.APIKeyEnv) != "" {
			mark = "✓"
		}
		hint := "API key"
		switch {
		case p.Local:
			hint = "local server, no key"
		case p.APIKeyEnv != "":
			hint = "API key or " + p.APIKeyEnv
		}
		b.WriteString(fmt.Sprintf("  %s %s  %s\n",
			m.styles.ToolGlyph.Render(mark), m.styles.ToolCall.Render(p.ID), m.styles.Hint.Render(hint)))
	}
	b.WriteString("  " + m.styles.Hint.Render("switch: /connect <provider>   ·   add a key: /connect <provider> <api-key>") + "\n")
	b.WriteString("  " + m.styles.Hint.Render("guided setup with masked key entry: run `vala connect` in your shell"))
	return strings.TrimRight(b.String(), "\n")
}

// storeInlineCredential persists a secret passed to /connect: a base URL for a
// local provider, otherwise an API key. It preserves any existing model choice.
func storeInlineCredential(info llm.ProviderInfo, secret string) error {
	store, err := auth.Load()
	if err != nil {
		return err
	}
	cred := auth.Credential{Type: "api"}
	if existing, ok := store.Get(info.ID); ok {
		cred = existing
	}
	if info.Local {
		cred.BaseURL = secret
	} else {
		cred.Key = secret
	}
	return store.Set(info.ID, cred)
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

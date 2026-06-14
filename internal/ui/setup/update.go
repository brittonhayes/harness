package setup

import (
	"fmt"
	"strings"

	"github.com/brittonhayes/vala/internal/auth"
	"github.com/brittonhayes/vala/internal/config"
	"github.com/brittonhayes/vala/internal/llm"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// hub row ids.
const (
	rowProvider = "provider"
	rowBrain    = "brain"
	rowEvidence = "evidence"
	rowStart    = "start"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd

	case oauthExchangedMsg:
		return m.onOAuthExchanged(msg)

	case notionCheckedMsg:
		return m.onNotionChecked(msg)

	case notionLoginDoneMsg:
		return m.onNotionLoginDone(msg)

	case notionProvisionedMsg:
		return m.onNotionProvisioned(msg)

	case evidenceValidatedMsg:
		m.evidence = append(m.evidence, msg.status)
		m.screen = screenEvidenceResult
		return m, nil

	case notionSearchValidatedMsg:
		return m.onNotionSearchValidated(msg)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.result.Quit = true
			return m, tea.Quit
		}
		return m.onKey(msg)
	}
	return m, nil
}

// onKey routes a key press to the active screen.
func (m model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	m.errMsg = "" // any deliberate action clears the last error

	switch m.screen {
	case screenHub, screenProviderPick, screenProviderAuth, screenBrainPick, screenEvidencePick:
		switch key {
		case "up", "k":
			m.sel.move(-1)
		case "down", "j":
			m.sel.move(1)
		case "esc":
			return m.onEsc()
		case "enter":
			return m.choose()
		}

	case screenProviderOAuth, screenProviderKey, screenProviderLocal, screenEvidenceForm, screenBrainNotionPage, screenBrainNotionMCP:
		if key == "esc" {
			return m.onEsc()
		}
		submitted, cmd := m.form.update(msg)
		if submitted {
			return m.submitForm()
		}
		return m, cmd

	case screenBrainNotionDone:
		if key == "enter" || key == "esc" {
			return m.toHub()
		}

	case screenEvidenceResult:
		if key == "enter" || key == "esc" {
			return m.startEvidence() // back to the evidence menu to add more or finish
		}
	}
	return m, nil
}

// onEsc backs out: from a sub-step to the hub, from the hub it finishes setup.
func (m model) onEsc() (tea.Model, tea.Cmd) {
	if m.screen == screenHub {
		return m, tea.Quit
	}
	return m.toHub()
}

// --- hub ---

// toHub rebuilds and shows the home screen, reflecting current status.
func (m model) toHub() (tea.Model, tea.Cmd) {
	m.sel = newSelector(
		choice{id: rowProvider, label: "Model provider", desc: m.providerStatus()},
		choice{id: rowBrain, label: "Brain (memory)", desc: m.brainStatus()},
		choice{id: rowEvidence, label: "Evidence sources", desc: m.evidenceStatus()},
		choice{id: rowStart, label: "Start hunting →", desc: "launch vala with this setup"},
	)
	m.screen = screenHub
	return m, nil
}

func (m model) providerStatus() string {
	if m.providerDone {
		return m.opts.Model
	}
	return "not connected"
}

func (m model) brainStatus() string {
	if m.result.BrainLocal {
		return "on-disk (new)"
	}
	if m.brainDone {
		if m.opts.Brain != "" {
			return m.opts.Brain
		}
		return "configured"
	}
	// A pre-existing summary on an unfinished row means a configured brain that
	// still needs attention (e.g. "Notion — needs repair"); surface it instead of
	// the ephemeral default so the operator sees why the wizard opened.
	if m.opts.Brain != "" {
		return m.opts.Brain
	}
	return "ephemeral — findings vanish on exit"
}

func (m model) evidenceStatus() string {
	names := m.evidenceNames()
	if len(names) == 0 {
		return "none — nothing to hunt in yet"
	}
	return strings.Join(names, ", ")
}

// evidenceNames merges sources already in config with ones connected this run.
func (m model) evidenceNames() []string {
	seen := map[string]bool{}
	var out []string
	add := func(n string) {
		if n != "" && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	for _, n := range m.opts.Evidence {
		add(n)
	}
	for _, e := range m.evidence {
		if e.OK() {
			add(e.Name)
		}
	}
	return out
}

// --- step entry ---

func (m model) startProvider() (tea.Model, tea.Cmd) {
	choices := make([]choice, 0)
	for _, p := range llm.Providers() {
		choices = append(choices, choice{id: p.ID, label: p.Name, desc: providerHint(p)})
	}
	m.sel = newSelector(choices...)
	m.screen = screenProviderPick
	return m, nil
}

func (m model) startBrain() (tea.Model, tea.Cmd) {
	m.sel = newSelector(
		choice{id: "local", label: "On-disk brain", desc: "durable, no account — recommended"},
		choice{id: "notion", label: "Notion brain", desc: "shared with your team (needs `ntn login`)"},
		choice{id: "skip", label: "Skip for now", desc: "ephemeral — findings vanish on exit"},
	)
	m.screen = screenBrainPick
	return m, nil
}

func (m model) startEvidence() (tea.Model, tea.Cmd) {
	m.sel = newSelector(
		choice{id: "scanner", label: "Scanner", desc: "scanner.dev security data lake (HTTPS)"},
		choice{id: "wiz", label: "Wiz", desc: "Wiz Security Graph (sign in with your browser)"},
		choice{id: "custom", label: "Custom (HTTP)", desc: "any hosted MCP server"},
		choice{id: "back", label: "← Back to setup", desc: m.evidenceStatus()},
	)
	m.screen = screenEvidencePick
	return m, nil
}

// --- selector choices ---

func (m model) choose() (tea.Model, tea.Cmd) {
	sel := m.sel.selected()
	switch m.screen {
	case screenHub:
		switch sel.id {
		case rowProvider:
			return m.startProvider()
		case rowBrain:
			return m.startBrain()
		case rowEvidence:
			return m.startEvidence()
		default: // start
			return m, tea.Quit
		}

	case screenProviderPick:
		info, _ := llm.Builtin(sel.id)
		m.provider = info
		switch {
		case info.Local:
			m.form = newForm(fieldSpec{key: "base_url", label: "Base URL", placeholder: info.BaseURL, value: info.BaseURL})
			m.screen = screenProviderLocal
		case info.OAuth:
			m.sel = newSelector(
				choice{id: "oauth", label: "Log in with your subscription", desc: "Claude Pro/Max — no API key stored"},
				choice{id: "key", label: "Paste an API key", desc: info.APIKeyEnv},
			)
			m.screen = screenProviderAuth
		default:
			m.form = newForm(fieldSpec{key: "key", label: info.Name + " API key", placeholder: "sk-…", secret: true})
			m.screen = screenProviderKey
		}
		return m, nil

	case screenProviderAuth:
		if sel.id == "oauth" {
			return m.beginOAuth()
		}
		m.form = newForm(fieldSpec{key: "key", label: m.provider.Name + " API key", placeholder: "sk-…", secret: true})
		m.screen = screenProviderKey
		return m, nil

	case screenBrainPick:
		switch sel.id {
		case "local":
			m.result.BrainLocal = true
			m.brainDone = true
			return m.toHub()
		case "notion":
			return m.startNotion()
		default:
			return m.toHub()
		}

	case screenEvidencePick:
		switch sel.id {
		case "scanner":
			m.evidencePreset = "scanner"
			m.form = newForm(fieldSpec{key: "url", label: "Scanner MCP URL", placeholder: "https://<you>.scanner.dev/mcp"})
			m.screen = screenEvidenceForm
		case "wiz":
			m.evidencePreset = "wiz"
			m.form = newForm(fieldSpec{key: "url", label: "Wiz MCP URL", placeholder: "https://mcp.app.wiz.io/", value: "https://mcp.app.wiz.io/"})
			m.screen = screenEvidenceForm
		case "custom":
			m.evidencePreset = "custom"
			m.form = newForm(
				fieldSpec{key: "name", label: "Name", placeholder: "splunk"},
				fieldSpec{key: "url", label: "MCP URL", placeholder: "https://…/mcp"},
				fieldSpec{key: "env", label: "API-key env var (optional)", placeholder: "SPLUNK_API_KEY"},
			)
			m.screen = screenEvidenceForm
		default: // back
			return m.toHub()
		}
		return m, nil
	}
	return m, nil
}

// --- form submissions ---

func (m model) submitForm() (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenProviderLocal:
		url := strings.TrimSpace(m.form.value("base_url"))
		if url == "" {
			url = m.provider.BaseURL
		}
		if err := m.saveProvider(auth.Credential{Type: "api", BaseURL: url, Model: m.provider.DefaultModel}); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		return m.providerConnected()

	case screenProviderKey:
		key := strings.TrimSpace(m.form.value("key"))
		if key == "" {
			m.errMsg = "no API key entered"
			return m, nil
		}
		if err := m.saveProvider(auth.Credential{Type: "api", Key: key, Model: m.provider.DefaultModel}); err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		return m.providerConnected()

	case screenProviderOAuth:
		code := strings.TrimSpace(m.form.value("code"))
		if code == "" {
			m.errMsg = "paste the code shown after you authorize"
			return m, nil
		}
		m.screen = screenProviderBusy
		m.busyLabel = "Signing in to " + m.provider.Name + "…"
		return m, exchangeOAuthCmd(m.ctx, code, m.oauthVerifier)

	case screenBrainNotionPage:
		page := strings.TrimSpace(m.form.value("page"))
		if page == "" {
			m.errMsg = "a Notion page ID is required to create the brain"
			return m, nil
		}
		m.screen = screenBrainNotionBusy
		m.busyLabel = "Provisioning the Notion brain…"
		return m, provisionNotionCmd(m.ctx, m.opts.Cwd, page)

	case screenBrainNotionMCP:
		return m.submitNotionSearch()

	case screenEvidenceForm:
		return m.submitEvidence()
	}
	return m, nil
}

func (m model) submitEvidence() (tea.Model, tea.Cmd) {
	var srv config.MCPServer
	switch m.evidencePreset {
	case "scanner":
		url := strings.TrimSpace(m.form.value("url"))
		if url == "" {
			m.errMsg = "a Scanner MCP URL is required"
			return m, nil
		}
		srv = config.MCPServer{Name: "scanner", Transport: "http", URL: url, APIKeyEnv: "SCANNER_API_KEY"}
	case "wiz":
		url := strings.TrimSpace(m.form.value("url"))
		if url == "" {
			m.errMsg = "a Wiz MCP URL is required"
			return m, nil
		}
		// Wiz authorizes in the browser on first use (MCP OAuth); no key to paste.
		srv = config.MCPServer{Name: "wiz", Transport: "http", URL: url, OAuth: true}
	default: // custom
		name := strings.TrimSpace(m.form.value("name"))
		url := strings.TrimSpace(m.form.value("url"))
		if name == "" || url == "" {
			m.errMsg = "name and URL are required"
			return m, nil
		}
		srv = config.MCPServer{Name: name, Transport: "http", URL: url, APIKeyEnv: strings.TrimSpace(m.form.value("env"))}
	}

	if err := config.SaveMCP(m.opts.Cwd, srv); err != nil {
		m.errMsg = "save .vala.json: " + err.Error()
		return m, nil
	}
	m.pendingServer = srv
	m.screen = screenEvidenceBusy
	m.busyLabel = "Connecting " + srv.Name + "…"
	return m, validateEvidenceCmd(m.ctx, srv)
}

// --- async results ---

func (m model) providerConnected() (tea.Model, tea.Cmd) {
	m.providerDone = true
	m.opts.Model = m.provider.ID + " · " + m.provider.DefaultModel
	return m.toHub()
}

func (m model) onOAuthExchanged(msg oauthExchangedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.errMsg = "login failed: " + msg.err.Error()
		m.screen = screenProviderAuth
		return m, nil
	}
	if err := m.saveProvider(msg.cred); err != nil {
		m.errMsg = err.Error()
		m.screen = screenProviderAuth
		return m, nil
	}
	return m.providerConnected()
}

// beginOAuth starts the subscription login: it mints the consent URL, opens the
// browser, and shows the paste-code field.
func (m model) beginOAuth() (tea.Model, tea.Cmd) {
	authz, err := authorizeOAuth()
	if err != nil {
		m.errMsg = "start login: " + err.Error()
		return m, nil
	}
	m.oauthVerifier = authz.Verifier
	openBrowser(authz.URL)
	m.notice = authz.URL
	m.form = newForm(fieldSpec{key: "code", label: "Paste the code shown after you authorize"})
	m.screen = screenProviderOAuth
	return m, nil
}

// --- Notion brain sub-flow ---

// startNotion enters the Notion brain flow: it checks the CLI is authenticated
// and probes any existing brain, then routes to login, repair, or a fresh
// provision based on what it finds.
func (m model) startNotion() (tea.Model, tea.Cmd) {
	m.screen = screenBrainNotionBusy
	m.busyLabel = "Checking Notion…"
	return m, checkNotionCmd(m.ctx, m.opts.Cwd, m.opts.Notion)
}

// startNotionSearch prompts for the MCP server backing Notion brain recall.
func (m model) startNotionSearch() (tea.Model, tea.Cmd) {
	m.form = newForm(fieldSpec{
		key:         "url",
		label:       "Notion MCP URL",
		placeholder: config.DefaultNotionMCPURL,
		value:       config.DefaultNotionMCPURL,
	})
	m.screen = screenBrainNotionMCP
	return m, nil
}

// onNotionChecked routes on the auth + health probe: log in if unauthenticated,
// repair in place when the database survives but stores are missing, report a
// healthy brain, or prompt for a parent page to provision a fresh one.
func (m model) onNotionChecked(msg notionCheckedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notionErr = msg.err
		m.screen = screenBrainNotionDone
		return m, nil
	}
	if !msg.authed {
		m.busyLabel = "Signing in to Notion…"
		m.screen = screenBrainNotionBusy
		return m, notionLoginCmd()
	}
	switch {
	case msg.databaseOK && len(msg.missing) == 0:
		if !m.opts.BrainSearchOK {
			return m.startNotionSearch()
		}
		// Already complete — nothing to do but confirm.
		m.brainDone = true
		m.opts.Brain = "Notion (shared)"
		m.notionMsg = "Notion brain is healthy — nothing to repair."
		m.screen = screenBrainNotionDone
		return m, nil
	case msg.databaseOK:
		// The database survives; add back the missing data sources in place.
		m.notionMissing = msg.missing
		m.busyLabel = "Repairing the Notion brain…"
		m.screen = screenBrainNotionBusy
		return m, repairNotionCmd(m.ctx, m.opts.Cwd, m.opts.Notion, msg.missing)
	default:
		// No usable database (never provisioned, or a legacy multi-DB layout):
		// provision a fresh single-database brain.
		m.form = newForm(fieldSpec{key: "page", label: "Notion page ID to create the brain under", placeholder: "page ID or URL"})
		m.screen = screenBrainNotionPage
		return m, nil
	}
}

// onNotionLoginDone re-probes after `ntn login` returns, so the flow continues
// from the same decision point now that the CLI is authenticated.
func (m model) onNotionLoginDone(msg notionLoginDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notionErr = fmt.Errorf("ntn login: %w", msg.err)
		m.screen = screenBrainNotionDone
		return m, nil
	}
	m.busyLabel = "Checking Notion…"
	m.screen = screenBrainNotionBusy
	return m, checkNotionCmd(m.ctx, m.opts.Cwd, m.opts.Notion)
}

// onNotionProvisioned records the outcome of a fresh provision or a repair. On
// success it flags BrainNotion so the caller scaffolds VALA.md.
func (m model) onNotionProvisioned(msg notionProvisionedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.notionErr = msg.err
		m.screen = screenBrainNotionDone
		return m, nil
	}
	m.result.BrainNotion = true
	if len(msg.repaired) > 0 {
		m.notionMsg = "Repaired " + strings.Join(msg.repaired, ", ") + "."
	} else {
		m.notionMsg = "Notion brain provisioned — one database, all stores."
	}
	if !m.opts.BrainSearchOK {
		return m.startNotionSearch()
	}
	m.brainDone = true
	m.opts.Brain = "Notion (shared)"
	m.screen = screenBrainNotionDone
	return m, nil
}

func (m model) submitNotionSearch() (tea.Model, tea.Cmd) {
	url := strings.TrimSpace(m.form.value("url"))
	if url == "" {
		url = config.DefaultNotionMCPURL
	}
	srv := config.MCPServer{Name: config.NotionSearchServerName, Transport: "http", URL: url, OAuth: true}
	if err := config.SaveMCP(m.opts.Cwd, srv); err != nil {
		m.errMsg = "save .vala.json: " + err.Error()
		return m, nil
	}
	m.pendingServer = srv
	m.screen = screenBrainNotionBusy
	m.busyLabel = "Connecting Notion MCP…"
	return m, validateNotionSearchCmd(m.ctx, srv)
}

func (m model) onNotionSearchValidated(msg notionSearchValidatedMsg) (tea.Model, tea.Cmd) {
	if !msg.status.OK() {
		m.notionErr = msg.status.Err
		m.screen = screenBrainNotionDone
		return m, nil
	}
	m.opts.BrainSearchOK = true
	m.brainDone = true
	m.opts.Brain = "Notion (shared)"
	if m.notionMsg == "" {
		m.notionMsg = "Notion brain is healthy."
	}
	m.notionMsg += " Recall is backed by Notion MCP search."
	m.screen = screenBrainNotionDone
	return m, nil
}

// saveProvider persists a provider credential and records the provider/model in
// .vala.json so the next session uses it.
func (m model) saveProvider(cred auth.Credential) error {
	store, err := auth.Load()
	if err != nil {
		return err
	}
	if err := store.Set(m.provider.ID, cred); err != nil {
		return err
	}
	return config.SaveProvider(m.opts.Cwd, m.provider.ID, m.provider.DefaultModel)
}

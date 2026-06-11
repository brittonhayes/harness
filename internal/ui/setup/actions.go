package setup

import (
	"context"
	"os"
	"os/exec"
	"runtime"

	"github.com/brittonhayes/vala/internal/auth"
	"github.com/brittonhayes/vala/internal/auth/oauth"
	"github.com/brittonhayes/vala/internal/brain"
	"github.com/brittonhayes/vala/internal/config"
	"github.com/brittonhayes/vala/internal/mcp"
	"github.com/brittonhayes/vala/internal/tools"
	tea "github.com/charmbracelet/bubbletea"
)

// oauthExchangedMsg carries the result of trading the pasted code for tokens.
type oauthExchangedMsg struct {
	cred auth.Credential
	err  error
}

// evidenceValidatedMsg carries the result of dialing a just-saved evidence
// source and listing its tools.
type evidenceValidatedMsg struct {
	status mcp.EvidenceStatus
}

// notionCheckedMsg reports whether the Notion CLI is authenticated and, when it
// is, the health of any configured brain: whether the parent database resolves
// and which stores are missing or unreachable.
type notionCheckedMsg struct {
	authed     bool
	databaseOK bool
	missing    []string
	err        error
}

// notionLoginDoneMsg signals `ntn login` (run with the TUI suspended) finished.
type notionLoginDoneMsg struct{ err error }

// notionProvisionedMsg carries the result of provisioning a fresh brain or
// repairing an existing one. repaired lists the stores added during a repair
// (empty for a fresh provision).
type notionProvisionedMsg struct {
	repaired []string
	err      error
}

// checkNotionCmd verifies the Notion CLI is authenticated and, if so, probes the
// configured brain (Verify) so the model can choose between provisioning fresh,
// repairing in place, or doing nothing.
func checkNotionCmd(ctx context.Context, cwd string, ids brain.DBIDs) tea.Cmd {
	return func() tea.Msg {
		store := &brain.NTN{Dir: cwd, DBs: ids}
		if err := store.Whoami(ctx); err != nil {
			return notionCheckedMsg{authed: false}
		}
		missing, databaseOK := store.Verify(ctx, ids)
		return notionCheckedMsg{authed: true, databaseOK: databaseOK, missing: missing}
	}
}

// notionLoginCmd suspends the TUI, runs `ntn login` (which opens the browser),
// and resumes — the one interactive step vala cannot do for the operator.
func notionLoginCmd() tea.Cmd {
	return tea.ExecProcess(exec.Command("ntn", "login"), func(err error) tea.Msg {
		return notionLoginDoneMsg{err: err}
	})
}

// provisionNotionCmd creates the single brain database and its data sources, then
// saves the resulting IDs to .vala.json.
func provisionNotionCmd(ctx context.Context, cwd, parentPage string) tea.Cmd {
	return func() tea.Msg {
		store := &brain.NTN{Dir: cwd}
		ids, err := store.Provision(ctx, parentPage)
		if err != nil {
			return notionProvisionedMsg{err: err}
		}
		if err := config.SaveNotion(cwd, ids); err != nil {
			return notionProvisionedMsg{err: err}
		}
		return notionProvisionedMsg{}
	}
}

// repairNotionCmd adds the missing data sources back under the existing brain
// database and saves the patched IDs to .vala.json.
func repairNotionCmd(ctx context.Context, cwd string, ids brain.DBIDs, missing []string) tea.Cmd {
	return func() tea.Msg {
		store := &brain.NTN{Dir: cwd, DBs: ids}
		patched, err := store.AddMissing(ctx, ids, missing)
		if err != nil {
			return notionProvisionedMsg{err: err}
		}
		if err := config.SaveNotion(cwd, patched); err != nil {
			return notionProvisionedMsg{err: err}
		}
		return notionProvisionedMsg{repaired: missing}
	}
}

// authorizeOAuth mints the consent URL and PKCE verifier for the subscription
// login. It is a thin wrapper so the model does not import the oauth package.
func authorizeOAuth() (oauth.Authorization, error) {
	return oauth.AnthropicAuthorize()
}

// exchangeOAuthCmd trades the pasted code for an OAuth credential off the UI
// goroutine.
func exchangeOAuthCmd(ctx context.Context, code, verifier string) tea.Cmd {
	return func() tea.Msg {
		tok, err := oauth.AnthropicExchange(ctx, code, verifier)
		if err != nil {
			return oauthExchangedMsg{err: err}
		}
		return oauthExchangedMsg{cred: auth.Credential{
			Type:    "oauth",
			Access:  tok.Access,
			Refresh: tok.Refresh,
			Expiry:  tok.Expiry.UnixMilli(),
		}}
	}
}

// validateEvidenceCmd dials a saved evidence source and discovers its tools, so
// the operator gets a live ✓/✗ instead of finding out at the first hunt. The
// secrets are resolved from the environment here, never persisted.
func validateEvidenceCmd(ctx context.Context, srv config.MCPServer) tea.Cmd {
	return func() tea.Msg {
		_, status := tools.ConnectEvidence(ctx, resolveServer(srv))
		return evidenceValidatedMsg{status: status}
	}
}

// resolveServer fills a persisted server's secrets from the environment: the
// bearer token for an HTTP server and any passthrough variables for a stdio one.
func resolveServer(srv config.MCPServer) mcp.ServerConfig {
	c := mcp.ServerConfig{
		Name:      srv.Name,
		Transport: srv.Transport,
		URL:       srv.URL,
		OAuth:     srv.OAuth,
		Command:   srv.Command,
		Args:      srv.Args,
	}
	if srv.APIKeyEnv != "" {
		c.APIKey = os.Getenv(srv.APIKeyEnv)
	}
	for _, name := range srv.EnvPassthrough {
		if v, ok := os.LookupEnv(name); ok {
			if c.Env == nil {
				c.Env = make(map[string]string)
			}
			c.Env[name] = v
		}
	}
	return c
}

// openBrowser best-effort opens a URL in the operator's default browser; the URL
// is always shown on screen as a fallback.
func openBrowser(url string) {
	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name = "open"
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler"}
	default:
		name = "xdg-open"
	}
	args = append(args, url)
	_ = exec.Command(name, args...).Start()
}

package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/brittonhayes/vala/internal/brain"
	"github.com/brittonhayes/vala/internal/tool"
)

//go:embed open_case.md
var openCaseDescription string

// CaseRunner works an alert through the governed response loop and returns a
// human-readable summary. It is satisfied by the respond engine, injected from
// the command layer; defining it here (rather than importing internal/respond)
// keeps internal/tools a leaf package with no governance-orchestration import.
type CaseRunner interface {
	RunCase(ctx context.Context, alert brain.Alert, title string) (summary string, err error)
}

// OpenCase hands an alert to the governed incident-response loop
// (plan → evidence → propose → approval → execute → report). That loop — its
// per-phase tool exposure, ledger-bound approval, and evidence lint — runs
// unchanged inside the CaseRunner; this tool is just the doorway into it from
// the unified harness. Class: case_write (opening a case is not itself an
// action; real-world actions are gated inside the loop).
type OpenCase struct {
	Runner CaseRunner
	Dir    string
}

func (t *OpenCase) Name() string        { return "open_case" }
func (t *OpenCase) Description() string { return openCaseDescription }
func (t *OpenCase) ReadOnly() bool      { return false }

func (t *OpenCase) Schema() tool.Schema {
	return tool.Schema{
		Properties: map[string]any{
			"path":     map[string]any{"type": "string", "description": "Path to an alert JSON file ({alert_id, source, severity, raw}). Use this or the inline fields."},
			"alert_id": map[string]any{"type": "string", "description": "Alert identifier (if not loading from a path)."},
			"source":   map[string]any{"type": "string", "description": "Where the alert came from, e.g. cloudtrail, guardduty."},
			"severity": map[string]any{"type": "string", "description": "Alert severity, e.g. low | medium | high | critical."},
			"raw":      map[string]any{"type": "string", "description": "The raw alert text / finding to investigate."},
			"title":    map[string]any{"type": "string", "description": "Case title (optional; defaults to the alert ID)."},
		},
	}
}

func (t *OpenCase) Run(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in struct {
		Path     string `json:"path"`
		AlertID  string `json:"alert_id"`
		Source   string `json:"source"`
		Severity string `json:"severity"`
		Raw      string `json:"raw"`
		Title    string `json:"title"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Errorf("invalid input: %v", err), nil
	}

	alert := brain.Alert{AlertID: in.AlertID, Source: in.Source, Severity: in.Severity, Raw: in.Raw}
	if in.Path != "" {
		path := in.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(t.Dir, path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return tool.Errorf("read alert: %v", err), nil
		}
		if err := json.Unmarshal(raw, &alert); err != nil {
			return tool.Errorf("parse alert %s: %v", in.Path, err), nil
		}
	}
	if alert.Raw == "" && alert.AlertID == "" {
		return tool.Errorf("provide an alert: either a path to an alert JSON file or the inline alert fields"), nil
	}

	title := in.Title
	if title == "" {
		title = alert.AlertID
	}
	if title == "" {
		title = "incident-" + alert.Source
	}

	summary, err := t.Runner.RunCase(ctx, alert, title)
	if err != nil {
		return tool.Errorf("case run failed: %v", err), nil
	}
	return tool.Text(summary), nil
}

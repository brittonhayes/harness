package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/brittonhayes/vala/internal/tool"
)

//go:embed choose.md
var chooseDescription string

// ChoiceMode names the selector behavior for an operator choice.
type ChoiceMode string

const (
	ChoiceSingle ChoiceMode = "single"
	ChoiceMulti  ChoiceMode = "multi"
)

// ChoiceOption is one selectable row in the operator choice UI.
type ChoiceOption struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Detail  string `json:"detail"`
	Default bool   `json:"default"`
}

// ChoiceRequest is the model-supplied prompt shown to the operator.
type ChoiceRequest struct {
	Question  string
	Mode      ChoiceMode
	Options   []ChoiceOption
	AllowChat bool
}

// ChoiceResponse is the operator's answer to a ChoiceRequest.
type ChoiceResponse struct {
	Selected []string
	Message  string
	Canceled bool
}

// ChoicePrompt renders a ChoiceRequest and blocks until the operator answers.
type ChoicePrompt func(context.Context, ChoiceRequest) (ChoiceResponse, error)

// Choose asks the operator to select one or more options through the REPL.
type Choose struct {
	Prompt ChoicePrompt
}

func (c *Choose) Name() string        { return "choose" }
func (c *Choose) Description() string { return chooseDescription }
func (c *Choose) ReadOnly() bool      { return true }

func (c *Choose) Schema() tool.Schema {
	return tool.Schema{
		Properties: map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The concise question shown above the selector.",
			},
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{string(ChoiceSingle), string(ChoiceMulti)},
				"description": "single for exactly one selected option, multi for a checklist.",
			},
			"options": map[string]any{
				"type":        "array",
				"description": "Selectable choices. Use stable short ids such as A, B, C or descriptive slugs.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id":      map[string]any{"type": "string", "description": "Stable id returned if selected."},
						"label":   map[string]any{"type": "string", "description": "Short label shown in the selector."},
						"detail":  map[string]any{"type": "string", "description": "Optional explanation or tradeoff."},
						"default": map[string]any{"type": "boolean", "description": "Preselect this option as the recommended default."},
					},
					"required": []string{"id", "label"},
				},
				"minItems": 1,
			},
			"allow_chat": map[string]any{
				"type":        "boolean",
				"description": "Whether the operator can answer with free-form instructions instead of selecting. Defaults to true.",
			},
		},
		Required: []string{"question", "options"},
	}
}

type chooseInput struct {
	Question  string         `json:"question"`
	Mode      string         `json:"mode"`
	Options   []ChoiceOption `json:"options"`
	AllowChat *bool          `json:"allow_chat"`
}

func (c *Choose) Run(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in chooseInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Errorf("invalid input: %v", err), nil
	}
	req, res := normalizeChoiceInput(in)
	if res.IsError {
		return res, nil
	}
	if c.Prompt == nil {
		return tool.Errorf("operator choice is unavailable outside the interactive REPL"), nil
	}
	ans, err := c.Prompt(ctx, req)
	if err != nil {
		return tool.Errorf("operator choice failed: %v", err), nil
	}
	return tool.Text(formatChoiceResponse(req, ans)), nil
}

func normalizeChoiceInput(in chooseInput) (ChoiceRequest, tool.Result) {
	question := strings.TrimSpace(in.Question)
	if question == "" {
		return ChoiceRequest{}, tool.Errorf("question is required")
	}
	mode := ChoiceMode(strings.ToLower(strings.TrimSpace(in.Mode)))
	if mode == "" {
		mode = ChoiceSingle
	}
	if mode != ChoiceSingle && mode != ChoiceMulti {
		return ChoiceRequest{}, tool.Errorf("mode must be %q or %q", ChoiceSingle, ChoiceMulti)
	}
	if len(in.Options) == 0 {
		return ChoiceRequest{}, tool.Errorf("at least one option is required")
	}
	options := make([]ChoiceOption, 0, len(in.Options))
	seen := map[string]bool{}
	for i, opt := range in.Options {
		id := strings.TrimSpace(opt.ID)
		label := strings.TrimSpace(opt.Label)
		if id == "" {
			id = label
		}
		if label == "" {
			label = id
		}
		if id == "" {
			return ChoiceRequest{}, tool.Errorf("options[%d] needs id or label", i)
		}
		if seen[id] {
			return ChoiceRequest{}, tool.Errorf("duplicate option id %q", id)
		}
		seen[id] = true
		options = append(options, ChoiceOption{
			ID:      id,
			Label:   label,
			Detail:  strings.TrimSpace(opt.Detail),
			Default: opt.Default,
		})
	}
	allowChat := true
	if in.AllowChat != nil {
		allowChat = *in.AllowChat
	}
	return ChoiceRequest{Question: question, Mode: mode, Options: options, AllowChat: allowChat}, tool.Result{}
}

func formatChoiceResponse(req ChoiceRequest, ans ChoiceResponse) string {
	if ans.Canceled {
		return "operator canceled the choice"
	}
	var lines []string
	if len(ans.Selected) > 0 {
		labels := make([]string, 0, len(ans.Selected))
		for _, id := range ans.Selected {
			labels = append(labels, choiceLabel(req.Options, id))
		}
		lines = append(lines, "operator selected: "+strings.Join(labels, ", "))
	}
	if msg := strings.TrimSpace(ans.Message); msg != "" {
		if len(lines) == 0 {
			lines = append(lines, "operator replied instead of selecting:")
		} else {
			lines = append(lines, "operator note:")
		}
		lines = append(lines, msg)
	}
	if len(lines) == 0 {
		return "operator submitted no selection"
	}
	return strings.Join(lines, "\n")
}

func choiceLabel(options []ChoiceOption, id string) string {
	for _, opt := range options {
		if opt.ID == id {
			if opt.Label == opt.ID {
				return opt.ID
			}
			return fmt.Sprintf("%s (%s)", opt.ID, opt.Label)
		}
	}
	return id
}

// ChoiceFromRegistry returns the registered choose tool, if present.
func ChoiceFromRegistry(reg *tool.Registry) *Choose {
	if reg == nil {
		return nil
	}
	t, ok := reg.Get("choose")
	if !ok {
		return nil
	}
	ch, _ := t.(*Choose)
	return ch
}

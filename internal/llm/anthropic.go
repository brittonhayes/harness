package llm

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// anthropicProvider speaks the Anthropic Messages API. It keeps Claude's native
// strengths — prompt-shaped tool use and the larger context windows — that a
// compatibility shim would lose.
type anthropicProvider struct {
	api           anthropic.Client
	model         string
	maxTokens     int64
	contextWindow int64
}

// newAnthropic builds an Anthropic provider. baseURL is optional (empty uses the
// SDK default endpoint), which lets it also drive Anthropic-compatible gateways.
func newAnthropic(apiKey, baseURL, model string, maxTokens, contextWindow int64) *anthropicProvider {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &anthropicProvider{
		api:           anthropic.NewClient(opts...),
		model:         model,
		maxTokens:     maxTokens,
		contextWindow: contextWindow,
	}
}

func (p *anthropicProvider) Model() string        { return p.model }
func (p *anthropicProvider) Provider() string     { return "anthropic" }
func (p *anthropicProvider) ContextWindow() int64 { return p.contextWindow }

// Complete renders the neutral conversation onto the Messages API and decodes
// the reply back into neutral blocks.
func (p *anthropicProvider) Complete(ctx context.Context, system string, messages []Message, tools []ToolDef) (*Response, error) {
	resp, err := p.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(p.model),
		MaxTokens: p.maxTokens,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  toAnthropicMessages(messages),
		Tools:     toAnthropicTools(tools),
	})
	if err != nil {
		return nil, err
	}
	return fromAnthropicMessage(resp), nil
}

// toAnthropicMessages converts neutral messages into Messages API params.
func toAnthropicMessages(messages []Message) []anthropic.MessageParam {
	out := make([]anthropic.MessageParam, 0, len(messages))
	for _, m := range messages {
		blocks := make([]anthropic.ContentBlockParamUnion, 0, len(m.Content))
		for _, b := range m.Content {
			switch b.Type {
			case BlockText:
				blocks = append(blocks, anthropic.NewTextBlock(b.Text))
			case BlockToolUse:
				blocks = append(blocks, anthropic.NewToolUseBlock(b.ID, b.Input, b.Name))
			case BlockToolResult:
				blocks = append(blocks, anthropic.NewToolResultBlock(b.ToolUseID, b.Content, b.IsError))
			}
		}
		if m.Role == RoleAssistant {
			out = append(out, anthropic.NewAssistantMessage(blocks...))
		} else {
			out = append(out, anthropic.NewUserMessage(blocks...))
		}
	}
	return out
}

// toAnthropicTools converts neutral tool definitions into Messages API tool
// params. A nil result is returned for an empty tool set so a tool-free call
// (e.g. summarization) sends no tools.
func toAnthropicTools(tools []ToolDef) []anthropic.ToolUnionParam {
	if len(tools) == 0 {
		return nil
	}
	out := make([]anthropic.ToolUnionParam, 0, len(tools))
	for _, t := range tools {
		props := t.Properties
		if props == nil {
			props = map[string]any{}
		}
		out = append(out, anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        t.Name,
				Description: anthropic.String(t.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: props,
					Required:   t.Required,
				},
			},
		})
	}
	return out
}

// fromAnthropicMessage decodes a Messages API reply into neutral blocks.
func fromAnthropicMessage(msg *anthropic.Message) *Response {
	if msg == nil {
		return &Response{}
	}
	out := &Response{
		Usage: Usage{
			InputTokens:  msg.Usage.InputTokens,
			OutputTokens: msg.Usage.OutputTokens,
		},
	}
	for _, b := range msg.Content {
		switch b.Type {
		case "text":
			out.Content = append(out.Content, TextBlock(b.Text))
		case "tool_use":
			out.Content = append(out.Content, ToolUseBlock(b.ID, b.Name, b.Input))
		}
	}
	return out
}

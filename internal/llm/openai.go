package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// openaiProvider speaks the OpenAI Chat Completions API. Because that wire
// format is the de-facto standard, one implementation — pointed at a different
// base URL — serves OpenAI, Google's OpenAI-compatible endpoint, OpenRouter,
// Groq, DeepSeek, xAI, and local servers (Ollama, LM Studio). Requests are
// non-streaming, matching the rest of vala's turn-at-a-time loop.
type openaiProvider struct {
	http     *http.Client
	provider string
	baseURL  string
	apiKey   string
	model    string

	maxTokens     int64
	contextWindow int64

	// reasoning models on OpenAI itself reject max_tokens and require
	// max_completion_tokens; compatible servers generally accept max_tokens.
	useCompletionTokens bool
}

func newOpenAI(p ProviderInfo, apiKey, baseURL, model string, maxTokens, contextWindow int64) *openaiProvider {
	return &openaiProvider{
		http:                http.DefaultClient,
		provider:            p.ID,
		baseURL:             strings.TrimRight(baseURL, "/"),
		apiKey:              apiKey,
		model:               model,
		maxTokens:           maxTokens,
		contextWindow:       contextWindow,
		useCompletionTokens: p.ID == "openai",
	}
}

func (p *openaiProvider) Model() string        { return p.model }
func (p *openaiProvider) Provider() string     { return p.provider }
func (p *openaiProvider) ContextWindow() int64 { return p.contextWindow }

// --- request/response wire shapes ---

type oaiMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	ToolCalls  []oaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
}

type oaiToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function oaiFunctionCall `json:"function"`
}

type oaiFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type oaiTool struct {
	Type     string      `json:"type"`
	Function oaiToolFunc `json:"function"`
}

type oaiToolFunc struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

type oaiRequest struct {
	Model               string       `json:"model"`
	Messages            []oaiMessage `json:"messages"`
	Tools               []oaiTool    `json:"tools,omitempty"`
	MaxTokens           int64        `json:"max_tokens,omitempty"`
	MaxCompletionTokens int64        `json:"max_completion_tokens,omitempty"`
}

type oaiResponse struct {
	Choices []struct {
		Message struct {
			Content   string        `json:"content"`
			ToolCalls []oaiToolCall `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int64 `json:"prompt_tokens"`
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Complete renders the neutral conversation onto Chat Completions and decodes
// the reply back into neutral blocks.
func (p *openaiProvider) Complete(ctx context.Context, system string, messages []Message, tools []ToolDef) (*Response, error) {
	reqBody := oaiRequest{
		Model:    p.model,
		Messages: toOpenAIMessages(system, messages),
		Tools:    toOpenAITools(tools),
	}
	if p.useCompletionTokens {
		reqBody.MaxCompletionTokens = p.maxTokens
	} else {
		reqBody.MaxTokens = p.maxTokens
	}

	buf, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var decoded oaiResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		// Non-JSON error bodies (gateways, local servers) surface verbatim.
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
		}
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if decoded.Error != nil {
		return nil, fmt.Errorf("%s", decoded.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if len(decoded.Choices) == 0 {
		return nil, fmt.Errorf("model returned no choices")
	}

	choice := decoded.Choices[0].Message
	out := &Response{
		Usage: Usage{
			InputTokens:  decoded.Usage.PromptTokens,
			OutputTokens: decoded.Usage.CompletionTokens,
		},
	}
	if strings.TrimSpace(choice.Content) != "" {
		out.Content = append(out.Content, TextBlock(choice.Content))
	}
	for _, tc := range choice.ToolCalls {
		out.Content = append(out.Content, ToolUseBlock(tc.ID, tc.Function.Name, json.RawMessage(tc.Function.Arguments)))
	}
	return out, nil
}

// toOpenAIMessages renders the neutral conversation as Chat Completions
// messages. The system prompt becomes the leading system message. Tool results,
// which vala carries inside a user message, are emitted as separate role:"tool"
// messages immediately after the assistant turn that called them — the order
// Chat Completions requires.
func toOpenAIMessages(system string, messages []Message) []oaiMessage {
	out := make([]oaiMessage, 0, len(messages)+1)
	if strings.TrimSpace(system) != "" {
		out = append(out, oaiMessage{Role: "system", Content: system})
	}
	for _, m := range messages {
		switch m.Role {
		case RoleAssistant:
			msg := oaiMessage{Role: "assistant"}
			for _, b := range m.Content {
				switch b.Type {
				case BlockText:
					msg.Content += b.Text
				case BlockToolUse:
					msg.ToolCalls = append(msg.ToolCalls, oaiToolCall{
						ID:   b.ID,
						Type: "function",
						Function: oaiFunctionCall{
							Name:      b.Name,
							Arguments: string(b.Input),
						},
					})
				}
			}
			out = append(out, msg)
		default: // user
			var text strings.Builder
			for _, b := range m.Content {
				switch b.Type {
				case BlockText:
					text.WriteString(b.Text)
				case BlockToolResult:
					out = append(out, oaiMessage{
						Role:       "tool",
						ToolCallID: b.ToolUseID,
						Content:    b.Content,
					})
				}
			}
			if s := text.String(); s != "" {
				out = append(out, oaiMessage{Role: "user", Content: s})
			}
		}
	}
	return out
}

// toOpenAITools converts neutral tool definitions into Chat Completions tools.
func toOpenAITools(tools []ToolDef) []oaiTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]oaiTool, 0, len(tools))
	for _, t := range tools {
		params := map[string]any{
			"type":       "object",
			"properties": t.Properties,
		}
		if params["properties"] == nil {
			params["properties"] = map[string]any{}
		}
		if len(t.Required) > 0 {
			params["required"] = t.Required
		}
		out = append(out, oaiTool{
			Type: "function",
			Function: oaiToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		})
	}
	return out
}

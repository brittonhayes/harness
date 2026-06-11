// Package llm is vala's provider-agnostic model layer. The rest of vala depends
// on a small neutral surface — Message, Block, ToolDef, Response — rather than
// any one vendor SDK. A Provider implements one wire protocol (Anthropic
// Messages or OpenAI Chat Completions); the registry maps a provider id to the
// protocol, endpoint, and auth it needs, so adding OpenAI, Google, OpenRouter,
// Groq, or a local Ollama/LM Studio server is configuration, not new code.
package llm

import (
	"context"
	"encoding/json"
)

// Role identifies who authored a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// BlockType discriminates the kind of content a Block carries.
type BlockType string

const (
	// BlockText is plain assistant or user prose.
	BlockText BlockType = "text"
	// BlockToolUse is the model asking to invoke a tool.
	BlockToolUse BlockType = "tool_use"
	// BlockToolResult is the result of a tool the model invoked, fed back in.
	BlockToolResult BlockType = "tool_result"
)

// Block is one piece of message content. The active fields depend on Type:
// BlockText uses Text; BlockToolUse uses ID, Name, and Input; BlockToolResult
// uses ToolUseID, Content, and IsError. Keeping a single neutral shape lets the
// agent loop carry a conversation that any protocol can render onto the wire.
type Block struct {
	Type BlockType

	// Text holds prose (BlockText).
	Text string

	// ID and Name identify a tool call; Input is its raw JSON arguments
	// (BlockToolUse).
	ID    string
	Name  string
	Input json.RawMessage

	// ToolUseID references the call this result answers; Content is the result
	// payload and IsError marks a failed call (BlockToolResult).
	ToolUseID string
	Content   string
	IsError   bool
}

// TextBlock builds a text content block.
func TextBlock(text string) Block { return Block{Type: BlockText, Text: text} }

// ToolUseBlock builds a tool-call block.
func ToolUseBlock(id, name string, input json.RawMessage) Block {
	return Block{Type: BlockToolUse, ID: id, Name: name, Input: input}
}

// ToolResultBlock builds a tool-result block.
func ToolResultBlock(toolUseID, content string, isError bool) Block {
	return Block{Type: BlockToolResult, ToolUseID: toolUseID, Content: content, IsError: isError}
}

// Message is one turn in the conversation.
type Message struct {
	Role    Role
	Content []Block
}

// UserMessage builds a user-role message from content blocks.
func UserMessage(blocks ...Block) Message {
	return Message{Role: RoleUser, Content: blocks}
}

// AssistantMessage builds an assistant-role message from content blocks.
func AssistantMessage(blocks ...Block) Message {
	return Message{Role: RoleAssistant, Content: blocks}
}

// UserText is the common case: a user message that is a single text block.
func UserText(text string) Message { return UserMessage(TextBlock(text)) }

// ToolDef describes a tool the model may call. Properties and Required are the
// JSON Schema (draft 2020-12) of the tool's input object.
type ToolDef struct {
	Name        string
	Description string
	Properties  map[string]any
	Required    []string
}

// Usage reports the token accounting for a single response. InputTokens covers
// the full prompt the model saw (history, tools, system prompt), a good proxy
// for context fullness.
type Usage struct {
	InputTokens  int64
	OutputTokens int64
}

// Response is a single assistant reply: the content blocks it produced and the
// token usage for the request.
type Response struct {
	Content []Block
	Usage   Usage
}

// Provider talks to one model over one wire protocol. Implementations are
// stateless across calls: the caller owns the conversation and passes the full
// history each turn, so the same history can move between providers.
type Provider interface {
	// Complete sends the system prompt, conversation, and available tools, and
	// returns the assistant's reply. A nil or empty tools slice means no tools.
	Complete(ctx context.Context, system string, messages []Message, tools []ToolDef) (*Response, error)
	// Model returns the active model id (without the provider prefix).
	Model() string
	// Provider returns the active provider id (e.g. "anthropic", "ollama").
	Provider() string
	// ContextWindow returns the model's usable context size in tokens, or 0 when
	// unknown, so the caller can decide when to auto-compact.
	ContextWindow() int64
}

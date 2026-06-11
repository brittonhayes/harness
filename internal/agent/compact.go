package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/brittonhayes/vala/internal/llm"
)

// continuationPreamble frames the compacted history so the model treats the
// summary as the source of truth for everything that came before.
const continuationPreamble = "This session is being continued from a previous conversation that ran out of context. " +
	"The conversation is summarized below. Continue the work seamlessly from where it left off, " +
	"using the summary as your source of truth for prior context.\n\n"

// summaryRequest is the user turn that asks the model to produce the recap. It
// rides on top of the existing history so the model summarizes what it has seen.
const summaryRequest = "Summarize our conversation so far using the required structured sections. " +
	"This summary will replace the full transcript, so it must be complete enough to continue the work without the original messages."

// Compact summarizes the conversation in history into a single structured recap
// and returns a new history seeded with that recap, plus the raw summary text
// for display. focus is optional extra guidance from the operator (the argument
// to /compact). An empty history is a no-op: it returns history unchanged with
// an empty summary and a nil error.
func (a *Agent) Compact(ctx context.Context, history []llm.Message, focus string) (newHistory []llm.Message, summary string, err error) {
	if len(history) == 0 {
		return history, "", nil
	}
	if a.llm == nil {
		return history, "", fmt.Errorf("no model provider connected — run /connect to choose one")
	}

	req := summaryRequest
	if f := strings.TrimSpace(focus); f != "" {
		req += "\n\nAdditional focus from the operator: " + f
	}
	messages := append(history, llm.UserText(req))

	// Summarization is a single, tool-free completion — never the tool loop.
	resp, err := a.llm.Complete(ctx, compactionSystemPrompt(), messages, nil)
	if err != nil {
		return history, "", fmt.Errorf("compaction request failed: %w", err)
	}

	var b strings.Builder
	for _, block := range resp.Content {
		if block.Type == llm.BlockText {
			b.WriteString(block.Text)
		}
	}
	summary = strings.TrimSpace(b.String())
	if summary == "" {
		return history, "", fmt.Errorf("compaction produced an empty summary")
	}

	return buildContinuationHistory(summary), summary, nil
}

// buildContinuationHistory seeds a fresh history with a single user message that
// embeds the summary behind the continuation preamble. A single user-role seed
// keeps the next Complete call well-formed and mirrors Claude Code's pattern.
func buildContinuationHistory(summary string) []llm.Message {
	return []llm.Message{
		llm.UserText(continuationPreamble + summary),
	}
}

// compactionSystemPrompt is the Claude Code-style summarization prompt: it asks
// for a detailed, technical, lossless recap organized into fixed sections so the
// work can continue in a fresh context window.
func compactionSystemPrompt() string {
	return `You are summarizing a software/security engineering conversation so it can be continued in a fresh context window with far fewer tokens. Produce a detailed, structured summary that preserves everything needed to continue the work without loss of fidelity. Be specific and technical; do not be vague. Use the following sections, each as a markdown heading:

## Primary Request and Intent
Capture the user's explicit requests and overall goals, verbatim where it matters. Include all distinct asks in the order they were made.

## Key Technical Concepts
List the technologies, frameworks, detection techniques, data sources, and domain concepts that came up (e.g. Sigma rules, AWS CloudTrail, MITRE ATT&CK techniques, query languages).

## Files and Resources
List every file, detection rule, Notion page, or external resource that was read, created, or modified. For each, note its path/identifier and a one-line description of why it mattered and its current state. Include the most important code or rule snippets verbatim if they are load-bearing for continuing.

## Errors and Fixes
List errors, failed commands, validation failures, and how each was resolved (or that it remains unresolved). Note any operator corrections to your approach.

## Problem Solving
Summarize the investigation reasoning, hypotheses tested, evidence gathered, and conclusions reached so far.

## Pending Tasks
List explicitly requested work that is not yet complete.

## Current Work
Describe precisely what was being worked on at the moment the conversation was summarized, including the specific file, rule, or query in progress.

## Next Step (optional)
If there is a clear, already-agreed next action, state it. Only include a next step that directly continues the most recent work; do not invent new directions.

Output only the summary. Do not add commentary before or after it.`
}

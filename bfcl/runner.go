package bfcl

import (
	"context"
	"fmt"

	loop "github.com/benaskins/axon-loop"
	tool "github.com/benaskins/axon-tool"
)

// Runner executes BFCL test cases through axon-loop, which handles
// the conversation loop including retries when the model doesn't
// produce tool calls on the first attempt.
type Runner struct {
	client   loop.LLMClient
	model    string
	maxTurns int
}

// NewRunner creates a Runner.
func NewRunner(client loop.LLMClient, model string) *Runner {
	return &Runner{
		client:   client,
		model:    model,
		maxTurns: 3,
	}
}

// Run sends a test case through axon-loop and returns the tool calls made.
func (r *Runner) Run(ctx context.Context, tc TestCase, cat Category) ([]loop.ToolCall, error) {
	if len(tc.Question) == 0 || len(tc.Question[0]) == 0 {
		return nil, fmt.Errorf("empty question")
	}

	msgs := ToMessages(tc.Question[0])
	toolDefs := ToTools(tc.Functions)

	// Build tool map for axon-loop (with stub executors).
	toolMap := make(map[string]tool.ToolDef, len(toolDefs))
	for _, td := range toolDefs {
		td := td
		td.Execute = func(ctx *tool.ToolContext, args map[string]any) tool.ToolResult {
			// Stub: return a plausible result so the loop can continue.
			return tool.ToolResult{Content: fmt.Sprintf("OK: %s called successfully", td.Name)}
		}
		toolMap[td.Name] = td
	}

	think := false
	req := &loop.Request{
		Model:         r.model,
		Messages:      msgs,
		Tools:         toolDefs,
		Think:         &think,
		Options:       map[string]any{"max_tokens": 300},
		MaxIterations: r.maxTurns,
	}

	var allToolCalls []loop.ToolCall
	cb := loop.Callbacks{
		OnToolUse: func(name string, args map[string]any) {
			allToolCalls = append(allToolCalls, loop.ToolCall{Name: name, Arguments: args})
		},
	}

	_, err := loop.Run(ctx, r.client, req, toolMap, nil, cb)
	if err != nil {
		// Max iterations exceeded is expected — it means the model
		// kept calling tools. Return what we collected.
		if len(allToolCalls) > 0 {
			return allToolCalls, nil
		}
		return nil, err
	}

	return allToolCalls, nil
}

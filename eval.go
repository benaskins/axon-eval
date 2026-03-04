package test

import (
	"fmt"
	"strings"
)

// CheckFunc evaluates a response and returns pass/fail with a reason.
type CheckFunc func(response string) (pass bool, reason string)

// EvalScenario is a test scenario with an expected outcome check.
type EvalScenario struct {
	Messages []Message
	Check    CheckFunc
}

// EvalResult holds the result of a single evaluation scenario.
type EvalResult struct {
	Messages []Message
	Response string
	Result   ChatResult
	Pass     bool
	Reason   string
}

// EvalReport holds the aggregate results of an evaluation run.
type EvalReport struct {
	Passed  int
	Failed  int
	Results []EvalResult
}

// Evaluate runs evaluation scenarios and checks responses against expected outcomes.
func (c *Client) Evaluate(description string, scenarios []EvalScenario) (*EvalReport, error) {
	runID := fmt.Sprintf("eval-%s", timeNowFormat())

	// Emit run_started
	if err := c.emitRunEvent("run_started", runID, description); err != nil {
		return nil, fmt.Errorf("emit run_started: %w", err)
	}

	report := &EvalReport{}

	for _, scenario := range scenarios {
		conversationID, err := c.createConversation()
		if err != nil {
			return nil, fmt.Errorf("create conversation: %w", err)
		}

		// Send all messages, collect the last response
		var lastResult ChatResult
		for _, msg := range scenario.Messages {
			chatResult, err := c.sendChat(msg, runID, conversationID)
			if err != nil {
				return nil, fmt.Errorf("eval chat: %w", err)
			}
			lastResult = *chatResult
		}

		pass, reason := scenario.Check(lastResult.Response)
		result := EvalResult{
			Messages: scenario.Messages,
			Response: lastResult.Response,
			Result:   lastResult,
			Pass:     pass,
			Reason:   reason,
		}
		report.Results = append(report.Results, result)

		if pass {
			report.Passed++
		} else {
			report.Failed++
		}
	}

	// Emit run_completed
	if err := c.emitRunEvent("run_completed", runID, description); err != nil {
		return nil, fmt.Errorf("emit run_completed: %w", err)
	}

	return report, nil
}

// ResponseContains returns a check that passes if the response contains the substring.
func ResponseContains(substr string) CheckFunc {
	return func(response string) (bool, string) {
		if strings.Contains(response, substr) {
			return true, ""
		}
		return false, fmt.Sprintf("response does not contain %q", substr)
	}
}

// ResponseMinLength returns a check that passes if the response has at least n characters.
func ResponseMinLength(n int) CheckFunc {
	return func(response string) (bool, string) {
		if len(response) >= n {
			return true, ""
		}
		return false, fmt.Sprintf("response length %d < minimum %d", len(response), n)
	}
}

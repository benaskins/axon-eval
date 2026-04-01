package eval

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// CriterionResult holds the result of evaluating a single criterion.
type CriterionResult struct {
	Criterion string  `json:"criterion"`
	Pass      bool    `json:"pass"`
	Score     float64 `json:"score"`
	Reason    string  `json:"reason,omitempty"`
}

// ScenarioGrade holds the aggregate grading results for a scenario.
type ScenarioGrade struct {
	Scenario string            `json:"scenario"`
	Results  []CriterionResult `json:"results"`
	Passed   int               `json:"passed"`
	Failed   int               `json:"failed"`
	Total    int               `json:"total"`
}

// JudgeResult holds the output of an LLM judge evaluation.
type JudgeResult struct {
	Pass   bool    `json:"pass"`
	Score  float64 `json:"score"`
	Reason string  `json:"reason"`
}

// Judge is an interface for LLM-based grading. See judge.go for implementation.
type Judge interface {
	Grade(ctx context.Context, response, idealResponse, criterion string) (*JudgeResult, error)
}

// GradeScenario evaluates a scenario's rubric criteria and auto-checks against a chat result.
// If judge is nil, llm_judge criteria are skipped.
func GradeScenario(ctx context.Context, scenario PlanScenario, result ChatResult, judge Judge) *ScenarioGrade {
	grade := &ScenarioGrade{Scenario: scenario.Name}

	// Auto-check: max_duration_ms
	if scenario.MaxDurationMs > 0 {
		pass := result.DurationMs <= scenario.MaxDurationMs
		r := CriterionResult{
			Criterion: "max_duration_ms",
			Pass:      pass,
			Score:     boolScore(pass),
		}
		if !pass {
			r.Reason = fmt.Sprintf("duration %dms exceeds max %dms", result.DurationMs, scenario.MaxDurationMs)
		}
		grade.Results = append(grade.Results, r)
	}

	// Auto-check: expected tools
	usedSet := toSet(result.ToolsUsed)
	for _, tool := range scenario.Tools.Expect {
		pass := usedSet[tool]
		r := CriterionResult{
			Criterion: "tool_used:" + tool,
			Pass:      pass,
			Score:     boolScore(pass),
		}
		if !pass {
			r.Reason = fmt.Sprintf("expected tool %q was not used", tool)
		}
		grade.Results = append(grade.Results, r)
	}

	// Auto-check: rejected tools
	for _, tool := range scenario.Tools.Reject {
		pass := !usedSet[tool]
		r := CriterionResult{
			Criterion: "tool_not_used:" + tool,
			Pass:      pass,
			Score:     boolScore(pass),
		}
		if !pass {
			r.Reason = fmt.Sprintf("rejected tool %q was used", tool)
		}
		grade.Results = append(grade.Results, r)
	}

	// Rubric criteria
	for _, c := range scenario.Rubric {
		r := evaluateCriterion(ctx, c, result, scenario.IdealResponse, judge)
		grade.Results = append(grade.Results, r)
	}

	// Tally
	for _, r := range grade.Results {
		grade.Total++
		if r.Pass {
			grade.Passed++
		} else {
			grade.Failed++
		}
	}

	return grade
}

func evaluateCriterion(ctx context.Context, c Criterion, result ChatResult, idealResponse string, judge Judge) CriterionResult {
	switch c.Type {
	case "contains":
		pass := strings.Contains(result.Response, c.Value)
		r := CriterionResult{Criterion: "contains:" + c.Value, Pass: pass, Score: boolScore(pass)}
		if !pass {
			r.Reason = fmt.Sprintf("response does not contain %q", c.Value)
		}
		return r

	case "not_contains":
		pass := !strings.Contains(result.Response, c.Value)
		r := CriterionResult{Criterion: "not_contains:" + c.Value, Pass: pass, Score: boolScore(pass)}
		if !pass {
			r.Reason = fmt.Sprintf("response contains %q", c.Value)
		}
		return r

	case "min_length":
		n, err := strconv.Atoi(c.Value)
		if err != nil {
			return CriterionResult{
				Criterion: "min_length:" + c.Value,
				Pass:      false,
				Score:     0,
				Reason:    fmt.Sprintf("invalid min_length value %q: %v", c.Value, err),
			}
		}
		pass := len(result.Response) >= n
		r := CriterionResult{Criterion: "min_length:" + c.Value, Pass: pass, Score: boolScore(pass)}
		if !pass {
			r.Reason = fmt.Sprintf("response length %d < minimum %d", len(result.Response), n)
		}
		return r

	case "max_length":
		n, err := strconv.Atoi(c.Value)
		if err != nil {
			return CriterionResult{
				Criterion: "max_length:" + c.Value,
				Pass:      false,
				Score:     0,
				Reason:    fmt.Sprintf("invalid max_length value %q: %v", c.Value, err),
			}
		}
		pass := len(result.Response) <= n
		r := CriterionResult{Criterion: "max_length:" + c.Value, Pass: pass, Score: boolScore(pass)}
		if !pass {
			r.Reason = fmt.Sprintf("response length %d > maximum %d", len(result.Response), n)
		}
		return r

	case "llm_judge":
		if judge == nil {
			return CriterionResult{
				Criterion: "llm_judge:" + c.Criterion,
				Pass:      false,
				Score:     0,
				Reason:    "skipped: no judge configured",
			}
		}
		jr, err := judge.Grade(ctx, result.Response, idealResponse, c.Criterion)
		if err != nil {
			return CriterionResult{
				Criterion: "llm_judge:" + c.Criterion,
				Pass:      false,
				Score:     0,
				Reason:    "judge error: " + err.Error(),
			}
		}
		return CriterionResult{
			Criterion: "llm_judge:" + c.Criterion,
			Pass:      jr.Pass,
			Score:     jr.Score,
			Reason:    jr.Reason,
		}

	default:
		return CriterionResult{
			Criterion: c.Type,
			Pass:      false,
			Score:     0,
			Reason:    fmt.Sprintf("unknown criterion type: %s", c.Type),
		}
	}
}

func boolScore(pass bool) float64 {
	if pass {
		return 1.0
	}
	return 0.0
}

func toSet(items []string) map[string]bool {
	s := make(map[string]bool, len(items))
	for _, item := range items {
		s[item] = true
	}
	return s
}

package eval

import (
	"context"
	"testing"
)

func TestGradeScenario_Contains(t *testing.T) {
	scenario := PlanScenario{
		Name:    "test",
		Message: "hello",
		Rubric: []Criterion{
			{Type: "contains", Value: "world"},
		},
	}
	result := ChatResult{Response: "hello world", DurationMs: 100, ToolsUsed: []string{}}

	grade := GradeScenario(context.Background(), scenario, result, nil)
	if len(grade.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(grade.Results))
	}
	if !grade.Results[0].Pass {
		t.Errorf("expected pass for contains 'world'")
	}
}

func TestGradeScenario_ContainsFail(t *testing.T) {
	scenario := PlanScenario{
		Name:    "test",
		Message: "hello",
		Rubric: []Criterion{
			{Type: "contains", Value: "xyz"},
		},
	}
	result := ChatResult{Response: "hello world", DurationMs: 100, ToolsUsed: []string{}}

	grade := GradeScenario(context.Background(), scenario, result, nil)
	if grade.Results[0].Pass {
		t.Errorf("expected fail for contains 'xyz'")
	}
	if grade.Results[0].Reason == "" {
		t.Error("expected reason on failure")
	}
}

func TestGradeScenario_NotContains(t *testing.T) {
	scenario := PlanScenario{
		Name:    "test",
		Message: "hello",
		Rubric: []Criterion{
			{Type: "not_contains", Value: "error"},
		},
	}
	result := ChatResult{Response: "all good", DurationMs: 100, ToolsUsed: []string{}}

	grade := GradeScenario(context.Background(), scenario, result, nil)
	if !grade.Results[0].Pass {
		t.Errorf("expected pass for not_contains 'error'")
	}
}

func TestGradeScenario_MinLength(t *testing.T) {
	scenario := PlanScenario{
		Name:    "test",
		Message: "hello",
		Rubric: []Criterion{
			{Type: "min_length", Value: "5"},
		},
	}
	result := ChatResult{Response: "hello world", DurationMs: 100, ToolsUsed: []string{}}

	grade := GradeScenario(context.Background(), scenario, result, nil)
	if !grade.Results[0].Pass {
		t.Errorf("expected pass for min_length 5 with 11 chars")
	}
}

func TestGradeScenario_MaxLength(t *testing.T) {
	scenario := PlanScenario{
		Name:    "test",
		Message: "hello",
		Rubric: []Criterion{
			{Type: "max_length", Value: "5"},
		},
	}
	result := ChatResult{Response: "hello world", DurationMs: 100, ToolsUsed: []string{}}

	grade := GradeScenario(context.Background(), scenario, result, nil)
	if grade.Results[0].Pass {
		t.Errorf("expected fail for max_length 5 with 11 chars")
	}
}

func TestGradeScenario_ToolExpect(t *testing.T) {
	scenario := PlanScenario{
		Name:    "test",
		Message: "hello",
		Tools: ToolExpect{
			Expect: []string{"check_weather"},
		},
	}
	result := ChatResult{Response: "sunny", DurationMs: 100, ToolsUsed: []string{"check_weather"}}

	grade := GradeScenario(context.Background(), scenario, result, nil)

	// Should have auto-generated tool_used criterion
	found := false
	for _, r := range grade.Results {
		if r.Criterion == "tool_used:check_weather" {
			found = true
			if !r.Pass {
				t.Error("expected pass for tool_used:check_weather")
			}
		}
	}
	if !found {
		t.Error("expected tool_used:check_weather criterion in results")
	}
}

func TestGradeScenario_ToolReject(t *testing.T) {
	scenario := PlanScenario{
		Name:    "test",
		Message: "hello",
		Tools: ToolExpect{
			Reject: []string{"web_search"},
		},
	}
	result := ChatResult{Response: "answer", DurationMs: 100, ToolsUsed: []string{"web_search"}}

	grade := GradeScenario(context.Background(), scenario, result, nil)

	found := false
	for _, r := range grade.Results {
		if r.Criterion == "tool_not_used:web_search" {
			found = true
			if r.Pass {
				t.Error("expected fail for tool_not_used:web_search when tool was used")
			}
		}
	}
	if !found {
		t.Error("expected tool_not_used:web_search criterion in results")
	}
}

func TestGradeScenario_MaxDuration(t *testing.T) {
	scenario := PlanScenario{
		Name:          "test",
		Message:       "hello",
		MaxDurationMs: 1000,
	}
	result := ChatResult{Response: "hi", DurationMs: 2000, ToolsUsed: []string{}}

	grade := GradeScenario(context.Background(), scenario, result, nil)

	found := false
	for _, r := range grade.Results {
		if r.Criterion == "max_duration_ms" {
			found = true
			if r.Pass {
				t.Error("expected fail for duration 2000 > max 1000")
			}
		}
	}
	if !found {
		t.Error("expected max_duration_ms criterion")
	}
}

func TestGradeScenario_LLMJudgeSkippedWithoutJudge(t *testing.T) {
	scenario := PlanScenario{
		Name:    "test",
		Message: "hello",
		Rubric: []Criterion{
			{Type: "llm_judge", Criterion: "Response is warm"},
		},
	}
	result := ChatResult{Response: "hi", DurationMs: 100, ToolsUsed: []string{}}

	grade := GradeScenario(context.Background(), scenario, result, nil)
	if len(grade.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(grade.Results))
	}
	if grade.Results[0].Pass {
		t.Error("expected skipped (not pass) for llm_judge without judge")
	}
	if grade.Results[0].Reason != "skipped: no judge configured" {
		t.Errorf("Reason = %q, want 'skipped: no judge configured'", grade.Results[0].Reason)
	}
}

func TestGradeScenario_PassRate(t *testing.T) {
	scenario := PlanScenario{
		Name:    "test",
		Message: "hello",
		Rubric: []Criterion{
			{Type: "contains", Value: "hello"},
			{Type: "contains", Value: "xyz"},
			{Type: "min_length", Value: "3"},
		},
	}
	result := ChatResult{Response: "hello world", DurationMs: 100, ToolsUsed: []string{}}

	grade := GradeScenario(context.Background(), scenario, result, nil)
	if grade.Passed != 2 {
		t.Errorf("Passed = %d, want 2", grade.Passed)
	}
	if grade.Failed != 1 {
		t.Errorf("Failed = %d, want 1", grade.Failed)
	}
	if grade.Total != 3 {
		t.Errorf("Total = %d, want 3", grade.Total)
	}
}

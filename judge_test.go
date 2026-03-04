package eval

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestOllamaJudge_Grade(t *testing.T) {
	generate := func(ctx context.Context, prompt string, temperature float64, maxTokens int) (string, error) {
		result := JudgeResult{
			Pass:   true,
			Score:  0.9,
			Reason: "Response is warm and friendly",
		}
		data, _ := json.Marshal(result)
		return string(data), nil
	}

	judge := NewOllamaJudge(generate)
	result, err := judge.Grade("Hello! How can I help?", "A warm greeting", "Response is warm and conversational")
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}

	if !result.Pass {
		t.Error("expected pass")
	}
	if result.Score != 0.9 {
		t.Errorf("Score = %f, want 0.9", result.Score)
	}
	if result.Reason != "Response is warm and friendly" {
		t.Errorf("Reason = %q", result.Reason)
	}
}

func TestOllamaJudge_GradeMarkdownFence(t *testing.T) {
	generate := func(ctx context.Context, prompt string, temperature float64, maxTokens int) (string, error) {
		return "```json\n{\"pass\":false,\"score\":0.3,\"reason\":\"Too brief\"}\n```", nil
	}

	judge := NewOllamaJudge(generate)
	result, err := judge.Grade("Hi", "A detailed greeting", "Response is detailed")
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}

	if result.Pass {
		t.Error("expected fail")
	}
	if result.Score != 0.3 {
		t.Errorf("Score = %f, want 0.3", result.Score)
	}
}

func TestOllamaJudge_GradeInvalidJSON(t *testing.T) {
	generate := func(ctx context.Context, prompt string, temperature float64, maxTokens int) (string, error) {
		return "not json at all", nil
	}

	judge := NewOllamaJudge(generate)
	_, err := judge.Grade("hi", "greeting", "warm")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestOllamaJudge_PromptContainsCriterion(t *testing.T) {
	var capturedPrompt string
	generate := func(ctx context.Context, prompt string, temperature float64, maxTokens int) (string, error) {
		capturedPrompt = prompt
		return `{"pass":true,"score":1.0,"reason":"ok"}`, nil
	}

	judge := NewOllamaJudge(generate)
	judge.Grade("response text", "ideal text", "is warm and friendly")

	if capturedPrompt == "" {
		t.Fatal("expected prompt to be captured")
	}
	for _, want := range []string{"response text", "ideal text", "is warm and friendly"} {
		if !strings.Contains(capturedPrompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func TestGradeScenario_WithJudge(t *testing.T) {
	generate := func(ctx context.Context, prompt string, temperature float64, maxTokens int) (string, error) {
		return `{"pass":true,"score":0.85,"reason":"Meets criterion"}`, nil
	}

	judge := NewOllamaJudge(generate)

	scenario := PlanScenario{
		Name:          "test",
		Message:       "hello",
		IdealResponse: "A warm greeting",
		Rubric: []Criterion{
			{Type: "llm_judge", Criterion: "Response is warm"},
			{Type: "contains", Value: "hello"},
		},
	}
	result := ChatResult{Response: "hello world", DurationMs: 100, ToolsUsed: []string{}}

	grade := GradeScenario(scenario, result, judge)
	if grade.Passed != 2 {
		t.Errorf("Passed = %d, want 2", grade.Passed)
	}

	// Check the LLM judge result
	judgeResult := grade.Results[0]
	if !judgeResult.Pass {
		t.Error("expected llm_judge to pass")
	}
	if judgeResult.Score != 0.85 {
		t.Errorf("Score = %f, want 0.85", judgeResult.Score)
	}
}

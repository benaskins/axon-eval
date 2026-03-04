package eval

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPlan(t *testing.T) {
	yaml := `
name: smoke test
scenarios:
  - name: greeting
    message: "Hello, how are you?"
    ideal_response: "A warm, friendly greeting"
    max_duration_ms: 5000
    tools:
      expect: []
      reject: []
    rubric:
      - type: min_length
        value: 50
      - type: llm_judge
        criterion: "Response is warm and conversational"

  - name: weather check
    message: "What's the weather in Melbourne?"
    ideal_response: "Uses check_weather tool and reports conditions"
    max_duration_ms: 10000
    tools:
      expect: [check_weather]
      reject: [web_search]
    rubric:
      - type: contains
        value: "Melbourne"
      - type: llm_judge
        criterion: "Response includes temperature"
`

	dir := t.TempDir()
	path := filepath.Join(dir, "smoke.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatal(err)
	}

	plan, err := LoadPlan(path)
	if err != nil {
		t.Fatalf("LoadPlan: %v", err)
	}

	if plan.Name != "smoke test" {
		t.Errorf("Name = %q, want %q", plan.Name, "smoke test")
	}
	if len(plan.Scenarios) != 2 {
		t.Fatalf("len(Scenarios) = %d, want 2", len(plan.Scenarios))
	}

	s := plan.Scenarios[0]
	if s.Name != "greeting" {
		t.Errorf("Scenario[0].Name = %q, want %q", s.Name, "greeting")
	}
	if s.Message != "Hello, how are you?" {
		t.Errorf("Scenario[0].Message = %q", s.Message)
	}
	if s.IdealResponse != "A warm, friendly greeting" {
		t.Errorf("Scenario[0].IdealResponse = %q", s.IdealResponse)
	}
	if s.MaxDurationMs != 5000 {
		t.Errorf("Scenario[0].MaxDurationMs = %d, want 5000", s.MaxDurationMs)
	}
	if len(s.Rubric) != 2 {
		t.Fatalf("Scenario[0].Rubric len = %d, want 2", len(s.Rubric))
	}
	if s.Rubric[0].Type != "min_length" {
		t.Errorf("Rubric[0].Type = %q, want min_length", s.Rubric[0].Type)
	}
	if s.Rubric[0].Value != "50" {
		t.Errorf("Rubric[0].Value = %q, want 50", s.Rubric[0].Value)
	}
	if s.Rubric[1].Type != "llm_judge" {
		t.Errorf("Rubric[1].Type = %q, want llm_judge", s.Rubric[1].Type)
	}
	if s.Rubric[1].Criterion != "Response is warm and conversational" {
		t.Errorf("Rubric[1].Criterion = %q", s.Rubric[1].Criterion)
	}

	s2 := plan.Scenarios[1]
	if len(s2.Tools.Expect) != 1 || s2.Tools.Expect[0] != "check_weather" {
		t.Errorf("Scenario[1].Tools.Expect = %v, want [check_weather]", s2.Tools.Expect)
	}
	if len(s2.Tools.Reject) != 1 || s2.Tools.Reject[0] != "web_search" {
		t.Errorf("Scenario[1].Tools.Reject = %v, want [web_search]", s2.Tools.Reject)
	}
}

func TestLoadPlan_Validation_MissingName(t *testing.T) {
	yaml := `
scenarios:
  - name: test
    message: "hello"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	_, err := LoadPlan(path)
	if err == nil {
		t.Fatal("expected error for missing plan name")
	}
}

func TestLoadPlan_Validation_MissingScenarioMessage(t *testing.T) {
	yaml := `
name: test
scenarios:
  - name: test
`
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	_, err := LoadPlan(path)
	if err == nil {
		t.Fatal("expected error for missing message")
	}
}

func TestLoadPlan_Validation_UnknownCriterionType(t *testing.T) {
	yaml := `
name: test
scenarios:
  - name: test
    message: "hello"
    rubric:
      - type: unknown_type
        value: "foo"
`
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte(yaml), 0644)

	_, err := LoadPlan(path)
	if err == nil {
		t.Fatal("expected error for unknown criterion type")
	}
}

func TestLoadPlan_FileNotFound(t *testing.T) {
	_, err := LoadPlan("/nonexistent/file.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

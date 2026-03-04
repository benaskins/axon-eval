package test

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Plan is a YAML-driven test plan containing named scenarios.
type Plan struct {
	Name      string         `yaml:"name"`
	Scenarios []PlanScenario `yaml:"scenarios"`
}

// PlanScenario is a single test scenario within a plan.
type PlanScenario struct {
	Name           string       `yaml:"name"`
	Message        string       `yaml:"message"`
	IdealResponse  string       `yaml:"ideal_response"`
	MaxDurationMs  int64        `yaml:"max_duration_ms"`
	Tools          ToolExpect   `yaml:"tools"`
	Rubric         []Criterion  `yaml:"rubric"`
}

// ToolExpect specifies which tools should or should not be used.
type ToolExpect struct {
	Expect []string `yaml:"expect"`
	Reject []string `yaml:"reject"`
}

// Criterion is a single rubric item for grading a scenario.
type Criterion struct {
	Type      string `yaml:"type"`
	Value     string `yaml:"value"`
	Criterion string `yaml:"criterion"`
}

// knownCriterionTypes lists all valid criterion types.
var knownCriterionTypes = map[string]bool{
	"contains":     true,
	"not_contains": true,
	"min_length":   true,
	"max_length":   true,
	"llm_judge":    true,
}

// LoadPlan reads and validates a YAML test plan from the given path.
func LoadPlan(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read plan: %w", err)
	}

	var plan Plan
	if err := yaml.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}

	if err := validatePlan(&plan); err != nil {
		return nil, fmt.Errorf("validate plan: %w", err)
	}

	return &plan, nil
}

func validatePlan(plan *Plan) error {
	if plan.Name == "" {
		return fmt.Errorf("plan name is required")
	}
	if len(plan.Scenarios) == 0 {
		return fmt.Errorf("plan must have at least one scenario")
	}
	for i, s := range plan.Scenarios {
		if s.Name == "" {
			return fmt.Errorf("scenario %d: name is required", i)
		}
		if s.Message == "" {
			return fmt.Errorf("scenario %q: message is required", s.Name)
		}
		for j, c := range s.Rubric {
			if !knownCriterionTypes[c.Type] {
				return fmt.Errorf("scenario %q, rubric %d: unknown criterion type %q", s.Name, j, c.Type)
			}
		}
	}
	return nil
}

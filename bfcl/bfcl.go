// Package bfcl runs the Berkeley Function Calling Leaderboard (BFCL)
// benchmark against any loop.LLMClient. It loads BFCL JSONL test cases,
// sends each through the client with the function definitions as tools,
// and scores the model's tool calls against the ground truth.
package bfcl

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	loop "github.com/benaskins/axon-loop"
	tool "github.com/benaskins/axon-tool"
)

// TestCase is a single BFCL test case loaded from JSONL.
type TestCase struct {
	ID        string              `json:"id"`
	Question  [][]Message         `json:"question"`
	Functions []FunctionDef       `json:"function"`
	Truth     []map[string]Params `json:"-"` // loaded from answer file
}

// Message is a chat message in BFCL format.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// FunctionDef is a BFCL function definition (OpenAI-style).
type FunctionDef struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Parameters  ParamSpec `json:"parameters"`
}

// ParamSpec describes function parameters.
type ParamSpec struct {
	Type       string                       `json:"type"`
	Properties map[string]PropertySpec      `json:"properties"`
	Required   []string                     `json:"required"`
}

// PropertySpec describes a single parameter.
type PropertySpec struct {
	Type        string                  `json:"type"`
	Description string                  `json:"description"`
	Enum        []any                   `json:"enum,omitempty"`
	Default     any                     `json:"default,omitempty"`
	Items       *PropertySpec           `json:"items,omitempty"`
	Properties  map[string]PropertySpec `json:"properties,omitempty"`
	Required    []string                `json:"required,omitempty"`
}

// Params maps parameter names to lists of acceptable values.
type Params map[string][]any

// Answer is a ground truth answer from the answer JSONL.
type Answer struct {
	ID    string              `json:"id"`
	Truth []map[string]Params `json:"ground_truth"`
}

// Result is the outcome of running one test case.
type Result struct {
	ID         string
	Pass       bool
	Expected   string
	Got        string
	DurationMs int64
	Error      string
}

// Summary aggregates results across all test cases.
type Summary struct {
	Total      int
	Passed     int
	Failed     int
	Errors     int
	Accuracy   float64
	AvgLatency float64
	Results    []Result
}

// LoadTestCases loads BFCL test cases and their ground truth answers.
func LoadTestCases(questionsPath, answersPath string) ([]TestCase, error) {
	cases, err := loadQuestions(questionsPath)
	if err != nil {
		return nil, fmt.Errorf("load questions: %w", err)
	}

	answers, err := loadAnswers(answersPath)
	if err != nil {
		return nil, fmt.Errorf("load answers: %w", err)
	}

	for i := range cases {
		if truth, ok := answers[cases[i].ID]; ok {
			cases[i].Truth = truth
		}
	}

	return cases, nil
}

func loadQuestions(path string) ([]TestCase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cases []TestCase
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var tc TestCase
		if err := json.Unmarshal(scanner.Bytes(), &tc); err != nil {
			return nil, fmt.Errorf("parse line: %w", err)
		}
		cases = append(cases, tc)
	}
	return cases, scanner.Err()
}

func loadAnswers(path string) (map[string][]map[string]Params, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	out := make(map[string][]map[string]Params)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var a Answer
		if err := json.Unmarshal(scanner.Bytes(), &a); err != nil {
			return nil, fmt.Errorf("parse answer: %w", err)
		}
		out[a.ID] = a.Truth
	}
	return out, scanner.Err()
}

// ToTools converts BFCL function definitions to axon-tool ToolDefs.
func ToTools(defs []FunctionDef) []tool.ToolDef {
	out := make([]tool.ToolDef, len(defs))
	for i, d := range defs {
		paramType := d.Parameters.Type
		if paramType == "dict" {
			paramType = "object"
		}
		out[i] = tool.ToolDef{
			Name:        d.Name,
			Description: d.Description,
			Parameters: tool.ParameterSchema{
				Type:       paramType,
				Required:   d.Parameters.Required,
				Properties: convertProperties(d.Parameters.Properties),
			},
		}
	}
	return out
}

func convertProperties(props map[string]PropertySpec) map[string]tool.PropertySchema {
	if len(props) == 0 {
		return nil
	}
	out := make(map[string]tool.PropertySchema, len(props))
	for name, p := range props {
		out[name] = convertProperty(p)
	}
	return out
}

func convertProperty(p PropertySpec) tool.PropertySchema {
	typ := p.Type
	if typ == "integer" || typ == "float" || typ == "double" {
		typ = "number"
	}
	if typ == "dict" {
		typ = "object"
	}
	ps := tool.PropertySchema{
		Type:        typ,
		Description: p.Description,
		Enum:        p.Enum,
		Default:     p.Default,
		Required:    p.Required,
		Properties:  convertProperties(p.Properties),
	}
	if p.Items != nil {
		items := convertProperty(*p.Items)
		ps.Items = &items
	}
	return ps
}

// ToMessages converts BFCL question messages to loop messages.
func ToMessages(msgs []Message) []loop.Message {
	out := make([]loop.Message, len(msgs))
	for i, m := range msgs {
		out[i] = loop.Message{Role: m.Role, Content: m.Content}
	}
	return out
}

// Category identifies the BFCL test category.
type Category string

const (
	Simple           Category = "simple"
	Multiple         Category = "multiple"
	Parallel         Category = "parallel"
	ParallelMultiple Category = "parallel_multiple"
	Irrelevance      Category = "irrelevance"
)

// Grade checks whether tool calls match the ground truth for a given category.
func Grade(calls []loop.ToolCall, truth []map[string]Params, category Category) bool {
	switch category {
	case Irrelevance:
		// Model should NOT make any tool calls.
		return len(calls) == 0
	case Parallel, ParallelMultiple:
		return gradeParallel(calls, truth)
	default:
		// Simple, Multiple: one expected call.
		return gradeSingle(calls, truth)
	}
}

func gradeSingle(calls []loop.ToolCall, truth []map[string]Params) bool {
	if len(truth) == 0 || len(calls) == 0 {
		return false
	}
	return matchCall(calls[0], truth[0])
}

func gradeParallel(calls []loop.ToolCall, truth []map[string]Params) bool {
	if len(truth) == 0 {
		return false
	}
	if len(calls) < len(truth) {
		return false
	}

	// Greedy match: for each expected call, find a matching actual call.
	used := make([]bool, len(calls))
	for _, expected := range truth {
		found := false
		for i, actual := range calls {
			if used[i] {
				continue
			}
			if matchCall(actual, expected) {
				used[i] = true
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func matchCall(actual loop.ToolCall, expected map[string]Params) bool {
	acceptableParams, ok := expected[actual.Name]
	if !ok {
		return false
	}

	for paramName, acceptableValues := range acceptableParams {
		actualValue, exists := actual.Arguments[paramName]
		if !exists {
			optional := false
			for _, v := range acceptableValues {
				if s, ok := v.(string); ok && s == "" {
					optional = true
					break
				}
			}
			if !optional {
				return false
			}
			continue
		}

		if !valueMatches(actualValue, acceptableValues) {
			return false
		}
	}

	return true
}

func valueMatches(actual any, acceptable []any) bool {
	for _, expected := range acceptable {
		if valuesEqual(actual, expected) {
			return true
		}
	}
	return false
}

func valuesEqual(a, b any) bool {
	// Try numeric comparison first — covers float64, int, and numeric strings.
	af, aIsNum := toFloatLoose(a)
	bf, bIsNum := toFloatLoose(b)
	if aIsNum && bIsNum {
		return af == bf
	}

	// String comparison (case-insensitive, normalize exponentiation).
	as, aIsStr := a.(string)
	bs, bIsStr := b.(string)
	if aIsStr && bIsStr {
		return strings.EqualFold(normalizeExpr(as), normalizeExpr(bs))
	}

	// Bool comparison.
	ab, aIsBool := a.(bool)
	bb, bIsBool := b.(bool)
	if aIsBool && bIsBool {
		return ab == bb
	}

	// Fallback: compare normalized string representations.
	return normalizeRepr(fmt.Sprintf("%v", a)) == normalizeRepr(fmt.Sprintf("%v", b))
}

// normalizeExpr normalizes equivalent mathematical notations so that
// e.g. "x^2" and "x**2" are treated as equal.
func normalizeExpr(s string) string {
	return strings.ReplaceAll(s, "^", "**")
}

// toFloatLoose converts numeric types and numeric strings to float64.
func toFloatLoose(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	}
	return 0, false
}

// normalizeRepr normalizes Go's %v formatting for cross-type comparison.
// Handles differences like "[1 3]" vs "[1, 3]" for slice representations.
func normalizeRepr(s string) string {
	s = strings.ReplaceAll(s, ", ", " ")
	return s
}

// FormatResult returns a human-readable summary line.
func FormatResult(r Result) string {
	status := "PASS"
	if !r.Pass {
		status = "FAIL"
	}
	if r.Error != "" {
		status = "ERR "
	}
	return fmt.Sprintf("[%s] %s (%dms) expected=%s got=%s %s",
		status, r.ID, r.DurationMs, r.Expected, r.Got, r.Error)
}

// FormatExpected returns a readable string of the expected function call.
func FormatExpected(truth []map[string]Params) string {
	if len(truth) == 0 {
		return "(none)"
	}
	for funcName, params := range truth[0] {
		parts := make([]string, 0, len(params))
		for k, v := range params {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v[0]))
		}
		return fmt.Sprintf("%s(%s)", funcName, strings.Join(parts, ", "))
	}
	return "(unknown)"
}

// FormatGot returns a readable string of the actual tool call.
func FormatGot(calls []loop.ToolCall) string {
	if len(calls) == 0 {
		return "(no tool call)"
	}
	tc := calls[0]
	parts := make([]string, 0, len(tc.Arguments))
	for k, v := range tc.Arguments {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return fmt.Sprintf("%s(%s)", tc.Name, strings.Join(parts, ", "))
}

// Elapsed returns milliseconds since start.
func Elapsed(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

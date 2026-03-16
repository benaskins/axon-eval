package bfcl

import (
	"os"
	"path/filepath"
	"testing"

	loop "github.com/benaskins/axon-loop"
)

func TestGrade_Simple_Pass(t *testing.T) {
	calls := []loop.ToolCall{{
		Name:      "get_weather",
		Arguments: map[string]any{"city": "Sydney"},
	}}
	truth := []map[string]Params{{
		"get_weather": {"city": {"Sydney"}},
	}}
	if !Grade(calls, truth, Simple) {
		t.Error("expected pass")
	}
}

func TestGrade_Simple_WrongFunction(t *testing.T) {
	calls := []loop.ToolCall{{
		Name:      "get_temperature",
		Arguments: map[string]any{"city": "Sydney"},
	}}
	truth := []map[string]Params{{
		"get_weather": {"city": {"Sydney"}},
	}}
	if Grade(calls, truth, Simple) {
		t.Error("expected fail for wrong function")
	}
}

func TestGrade_Simple_WrongValue(t *testing.T) {
	calls := []loop.ToolCall{{
		Name:      "get_weather",
		Arguments: map[string]any{"city": "Melbourne"},
	}}
	truth := []map[string]Params{{
		"get_weather": {"city": {"Sydney"}},
	}}
	if Grade(calls, truth, Simple) {
		t.Error("expected fail for wrong value")
	}
}

func TestGrade_Simple_CaseInsensitive(t *testing.T) {
	calls := []loop.ToolCall{{
		Name:      "search",
		Arguments: map[string]any{"query": "hello"},
	}}
	truth := []map[string]Params{{
		"search": {"query": {"Hello"}},
	}}
	if !Grade(calls, truth, Simple) {
		t.Error("expected case-insensitive match")
	}
}

func TestGrade_Simple_NumericMatch(t *testing.T) {
	calls := []loop.ToolCall{{
		Name:      "calc",
		Arguments: map[string]any{"x": float64(10)},
	}}
	truth := []map[string]Params{{
		"calc": {"x": {float64(10)}},
	}}
	if !Grade(calls, truth, Simple) {
		t.Error("expected numeric match")
	}
}

func TestGrade_Simple_OptionalMissing(t *testing.T) {
	calls := []loop.ToolCall{{
		Name:      "func",
		Arguments: map[string]any{"required": "yes"},
	}}
	truth := []map[string]Params{{
		"func": {"required": {"yes"}, "optional": {""}},
	}}
	if !Grade(calls, truth, Simple) {
		t.Error("expected pass when optional param missing")
	}
}

func TestGrade_Simple_NoCalls(t *testing.T) {
	truth := []map[string]Params{{
		"func": {"x": {"1"}},
	}}
	if Grade(nil, truth, Simple) {
		t.Error("expected fail with no calls")
	}
}

func TestGrade_Parallel_Pass(t *testing.T) {
	calls := []loop.ToolCall{
		{Name: "play", Arguments: map[string]any{"artist": "Taylor Swift"}},
		{Name: "play", Arguments: map[string]any{"artist": "Maroon 5"}},
	}
	truth := []map[string]Params{
		{"play": {"artist": {"Taylor Swift"}}},
		{"play": {"artist": {"Maroon 5"}}},
	}
	if !Grade(calls, truth, Parallel) {
		t.Error("expected pass")
	}
}

func TestGrade_Parallel_TooFewCalls(t *testing.T) {
	calls := []loop.ToolCall{
		{Name: "play", Arguments: map[string]any{"artist": "Taylor Swift"}},
	}
	truth := []map[string]Params{
		{"play": {"artist": {"Taylor Swift"}}},
		{"play": {"artist": {"Maroon 5"}}},
	}
	if Grade(calls, truth, Parallel) {
		t.Error("expected fail with too few calls")
	}
}

func TestGrade_Irrelevance_NoCalls(t *testing.T) {
	if !Grade(nil, nil, Irrelevance) {
		t.Error("expected pass when no calls made")
	}
}

func TestGrade_Irrelevance_UnwantedCall(t *testing.T) {
	calls := []loop.ToolCall{
		{Name: "func", Arguments: map[string]any{}},
	}
	if Grade(calls, nil, Irrelevance) {
		t.Error("expected fail when call made on irrelevance")
	}
}

func TestToTools(t *testing.T) {
	defs := []FunctionDef{{
		Name:        "calc",
		Description: "Calculate",
		Parameters: ParamSpec{
			Type:     "dict",
			Required: []string{"x"},
			Properties: map[string]PropertySpec{
				"x": {Type: "integer", Description: "value"},
			},
		},
	}}
	tools := ToTools(defs)
	if len(tools) != 1 {
		t.Fatalf("got %d tools", len(tools))
	}
	if tools[0].Parameters.Type != "object" {
		t.Errorf("type = %q, want object", tools[0].Parameters.Type)
	}
	if tools[0].Parameters.Properties["x"].Type != "number" {
		t.Errorf("x type = %q, want number", tools[0].Parameters.Properties["x"].Type)
	}
}

func TestToMessages(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hello"}}
	got := ToMessages(msgs)
	if len(got) != 1 || got[0].Role != "user" || got[0].Content != "hello" {
		t.Errorf("got %+v", got)
	}
}

func TestFormatExpected(t *testing.T) {
	truth := []map[string]Params{{
		"func": {"x": {float64(1)}},
	}}
	got := FormatExpected(truth)
	if got == "(none)" || got == "(unknown)" {
		t.Errorf("got %q", got)
	}
}

func TestFormatGot_NoCalls(t *testing.T) {
	got := FormatGot(nil)
	if got != "(no tool call)" {
		t.Errorf("got %q", got)
	}
}

func TestLoadTestCases(t *testing.T) {
	// Write temp JSONL files
	dir := t.TempDir()
	qPath := filepath.Join(dir, "q.json")
	aPath := filepath.Join(dir, "a.json")

	os.WriteFile(qPath, []byte(`{"id":"test_0","question":[[{"role":"user","content":"hi"}]],"function":[{"name":"f","description":"d","parameters":{"type":"dict","properties":{"x":{"type":"string","description":"val"}},"required":["x"]}}]}`+"\n"), 0644)
	os.WriteFile(aPath, []byte(`{"id":"test_0","ground_truth":[{"f":{"x":["hello"]}}]}`+"\n"), 0644)

	cases, err := LoadTestCases(qPath, aPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cases) != 1 {
		t.Fatalf("got %d cases", len(cases))
	}
	if cases[0].ID != "test_0" {
		t.Errorf("id = %q", cases[0].ID)
	}
	if len(cases[0].Truth) != 1 {
		t.Errorf("no truth loaded")
	}
}

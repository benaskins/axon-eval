package eval

import (
	"testing"

	fact "github.com/benaskins/axon-fact"
)

func TestBFCLResultSchema(t *testing.T) {
	s := BFCLResultSchema()
	if s.Name != "eval_bfcl" {
		t.Errorf("name = %q", s.Name)
	}
	if len(s.Fields) != 13 {
		t.Errorf("expected 13 fields, got %d", len(s.Fields))
	}

	// Verify key fields exist with correct types.
	checks := map[string]fact.FieldType{
		"timestamp":   fact.DateTime64,
		"model":       fact.LowCardinalityString,
		"category":    fact.LowCardinalityString,
		"pass":        fact.Bool,
		"duration_ms": fact.UInt32,
		"parameters":  fact.JSON,
	}
	for name, wantType := range checks {
		f, ok := s.FieldByName(name)
		if !ok {
			t.Errorf("missing field %q", name)
			continue
		}
		if f.Type != wantType {
			t.Errorf("field %q: type = %d, want %d", name, f.Type, wantType)
		}
	}

	if len(s.OrderBy) != 3 || s.OrderBy[0] != "model" {
		t.Errorf("OrderBy = %v", s.OrderBy)
	}
}

func TestLuthierResultSchema(t *testing.T) {
	s := LuthierResultSchema()
	if s.Name != "eval_luthier" {
		t.Errorf("name = %q", s.Name)
	}
	if len(s.Fields) != 14 {
		t.Errorf("expected 14 fields, got %d", len(s.Fields))
	}

	checks := map[string]fact.FieldType{
		"overall_score":    fact.Float32,
		"runs":             fact.UInt16,
		"total_duration_ms": fact.UInt32,
	}
	for name, wantType := range checks {
		f, ok := s.FieldByName(name)
		if !ok {
			t.Errorf("missing field %q", name)
			continue
		}
		if f.Type != wantType {
			t.Errorf("field %q: type = %d, want %d", name, f.Type, wantType)
		}
	}

	if len(s.OrderBy) != 2 || s.OrderBy[0] != "model" {
		t.Errorf("OrderBy = %v", s.OrderBy)
	}
}

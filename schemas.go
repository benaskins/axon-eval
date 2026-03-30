package eval

import fact "github.com/benaskins/axon-fact"

// BFCLResultSchema returns the fact.Schema for per-case BFCL eval results.
// One row per test case, ordered by (model, category, timestamp).
func BFCLResultSchema() fact.Schema {
	return fact.Schema{
		Name: "eval_bfcl",
		Fields: []fact.Field{
			{Name: "timestamp", Type: fact.DateTime64},
			{Name: "run_id", Type: fact.String},
			{Name: "model", Type: fact.LowCardinalityString},
			{Name: "provider", Type: fact.LowCardinalityString},
			{Name: "category", Type: fact.LowCardinalityString},
			{Name: "case_id", Type: fact.String},
			{Name: "pass", Type: fact.Bool},
			{Name: "error", Type: fact.String},
			{Name: "expected", Type: fact.String},
			{Name: "got", Type: fact.String},
			{Name: "duration_ms", Type: fact.UInt32},
			{Name: "prompt_hash", Type: fact.String},
			{Name: "parameters", Type: fact.JSON},
		},
		OrderBy: []string{"model", "category", "timestamp"},
	}
}

// LuthierResultSchema returns the fact.Schema for per-run luthier eval results.
// One row per evaluation run, ordered by (model, timestamp).
func LuthierResultSchema() fact.Schema {
	return fact.Schema{
		Name: "eval_luthier",
		Fields: []fact.Field{
			{Name: "timestamp", Type: fact.DateTime64},
			{Name: "run_id", Type: fact.String},
			{Name: "model", Type: fact.LowCardinalityString},
			{Name: "provider", Type: fact.LowCardinalityString},
			{Name: "runs", Type: fact.UInt16},
			{Name: "module_score", Type: fact.Float32},
			{Name: "boundary_score", Type: fact.Float32},
			{Name: "gap_score", Type: fact.Float32},
			{Name: "plan_step_score", Type: fact.Float32},
			{Name: "overall_score", Type: fact.Float32},
			{Name: "total_duration_ms", Type: fact.UInt32},
			{Name: "mean_duration_ms", Type: fact.UInt32},
			{Name: "prompt_hash", Type: fact.String},
			{Name: "parameters", Type: fact.JSON},
		},
		OrderBy: []string{"model", "timestamp"},
	}
}

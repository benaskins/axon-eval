// eval-ingest reads a JSON RunReport from stdin and records each result
// as a fact via the Pipeline to ClickHouse.
//
// Usage:
//
//	bfcl-run -dir bfcl/ -model qwen3.5 | eval-ingest
//	eval-ingest < results.json
//	eval-ingest -clickhouse http://localhost:8123
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	fact "github.com/benaskins/axon-fact"
	"github.com/benaskins/axon-eval"
	"github.com/benaskins/axon-eval/bfcl"
	look "github.com/benaskins/axon-look"
)

func main() {
	chURL := flag.String("clickhouse", "http://localhost:8123", "ClickHouse HTTP URL")
	flag.Parse()

	// Read stdin.
	var report bfcl.RunReport
	if err := json.NewDecoder(os.Stdin).Decode(&report); err != nil {
		slog.Error("decode stdin", "error", err)
		os.Exit(1)
	}

	if len(report.Results) == 0 {
		slog.Error("no results in report")
		os.Exit(1)
	}

	slog.Info("ingesting report",
		"run_id", report.RunID,
		"model", report.Model,
		"provider", report.Provider,
		"results", len(report.Results),
	)

	// Set up ClickHouse materializer.
	ch := look.NewClickHouse(*chURL)
	mat := look.NewCHMaterializer(ch)

	pipeline := fact.NewPipeline(fact.WithMaterializer(mat))

	ctx := context.Background()

	// Ensure schema exists.
	schema := eval.BFCLResultSchema()
	if err := pipeline.EnsureSchemas(ctx, schema); err != nil {
		slog.Error("ensure schema", "error", err)
		os.Exit(1)
	}

	// Convert results to facts.
	params, err := json.Marshal(report.Parameters)
	if err != nil {
		slog.Error("marshal parameters", "error", err)
		os.Exit(1)
	}
	now := time.Now().UTC()

	facts := make([]fact.Fact, 0, len(report.Results))
	for _, r := range report.Results {
		facts = append(facts, fact.Fact{
			Schema: "eval_bfcl",
			Data: map[string]any{
				"timestamp":   now,
				"run_id":      report.RunID,
				"model":       report.Model,
				"provider":    report.Provider,
				"category":    string(r.Category),
				"case_id":     r.ID,
				"pass":        r.Pass,
				"error":       r.Error,
				"expected":    r.Expected,
				"got":         r.Got,
				"duration_ms": uint32(r.DurationMs),
				"prompt_hash": "", // TODO: step 8 prompt store
				"parameters":  string(params),
			},
		})
	}

	// Record all facts.
	if err := pipeline.Record(ctx, facts...); err != nil {
		slog.Error("record facts", "error", err)
		os.Exit(1)
	}

	slog.Info("ingestion complete",
		"run_id", report.RunID,
		"facts", len(facts),
	)

	// Print summary.
	fmt.Fprintf(os.Stderr, "Ingested %d results for run %s (model=%s, provider=%s)\n",
		len(facts), report.RunID, report.Model, report.Provider)
}

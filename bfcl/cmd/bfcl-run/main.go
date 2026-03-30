// bfcl-run executes the full BFCL benchmark against a loop.LLMClient
// and reports per-category and overall accuracy.
//
// Text progress and summaries go to stderr.
// Structured JSON (RunReport) goes to stdout for piping to eval-ingest.
//
// Usage:
//
//	bfcl-run -dir bfcl/ -url http://localhost:8091 -model qwen3.5-122b
//	bfcl-run -dir bfcl/ -category simple -limit 20 -v
//	bfcl-run -dir bfcl/ -workers 10 | eval-ingest
package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benaskins/axon-eval/bfcl"
	loop "github.com/benaskins/axon-loop"
	"github.com/benaskins/axon-talk/openai"
)

type categorySpec struct {
	name      bfcl.Category
	questions string
	answers   string
}

type job struct {
	index int
	tc    bfcl.TestCase
	cat   bfcl.Category
}

func main() {
	var (
		dir       = flag.String("dir", "", "directory containing BFCL JSONL files")
		model     = flag.String("model", "@cf/qwen/qwen3-30b-a3b-fp8", "model identifier")
		provider  = flag.String("provider", "cloudflare", "provider name (cloudflare, local, anthropic)")
		baseURL   = flag.String("url", "", "AI Gateway base URL (default: from env)")
		token     = flag.String("token", "", "API token (default: from env)")
		limit     = flag.Int("limit", 0, "max test cases per category (0 = all)")
		category  = flag.String("category", "", "run only this category")
		workers   = flag.Int("workers", 10, "concurrent workers")
		useLoop   = flag.Bool("loop", false, "use axon-loop (retries, tool stub execution)")
		useStream = flag.Bool("stream", false, "enable SSE streaming")
		verbose   = flag.Bool("v", false, "print each result to stderr")
	)
	flag.Parse()

	if *dir == "" {
		fmt.Fprintf(os.Stderr, "usage: bfcl-run -dir DIR\n")
		os.Exit(1)
	}

	if *baseURL == "" {
		accountID := os.Getenv("CLOUDFLARE_ACCOUNT_ID")
		*baseURL = "https://gateway.ai.cloudflare.com/v1/" + accountID + "/axon-gate/workers-ai"
	}
	if *token == "" {
		*token = os.Getenv("CLOUDFLARE_AXON_GATE_TOKEN")
	}

	categories := []categorySpec{
		{bfcl.Simple, "simple.json", "answer_simple.json"},
		{bfcl.Multiple, "multiple.json", "answer_multiple.json"},
		{bfcl.Parallel, "parallel.json", "answer_parallel.json"},
		{bfcl.ParallelMultiple, "parallel_multiple.json", "answer_parallel_multiple.json"},
		{bfcl.Irrelevance, "irrelevance.json", ""},
	}

	if *category != "" {
		filtered := make([]categorySpec, 0, 1)
		for _, c := range categories {
			if string(c.name) == *category {
				filtered = append(filtered, c)
			}
		}
		if len(filtered) == 0 {
			fmt.Fprintf(os.Stderr, "unknown category: %s\n", *category)
			os.Exit(1)
		}
		categories = filtered
	}

	client := openai.NewClient(*baseURL, *token)
	runID := newRunID()

	var allResults []bfcl.Result

	for _, cat := range categories {
		qPath := filepath.Join(*dir, cat.questions)
		if _, err := os.Stat(qPath); err != nil {
			fmt.Fprintf(os.Stderr, "\n--- %s: skipped (file not found) ---\n", cat.name)
			continue
		}

		aPath := ""
		if cat.answers != "" {
			aPath = filepath.Join(*dir, cat.answers)
		}

		cases, err := loadCategory(qPath, aPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load %s: %v\n", cat.name, err)
			continue
		}

		if *limit > 0 && *limit < len(cases) {
			cases = cases[:*limit]
		}

		fmt.Fprintf(os.Stderr, "\n--- %s (%d cases, %d workers) ---\n", cat.name, len(cases), *workers)

		results := runCategory(client, *model, cat.name, cases, *workers, *useLoop, *useStream)

		passed, failed, errors := 0, 0, 0
		var catLatency int64
		for _, r := range results {
			catLatency += r.DurationMs
			if r.Error != "" {
				errors++
			} else if r.Pass {
				passed++
			} else {
				failed++
			}
		}

		if *verbose {
			for _, r := range results {
				fmt.Fprintln(os.Stderr, bfcl.FormatResult(r))
			}
		} else {
			for i, r := range results {
				mark := "."
				if !r.Pass {
					mark = "x"
				}
				if r.Error != "" {
					mark = "E"
				}
				fmt.Fprint(os.Stderr, mark)
				if (i+1)%50 == 0 {
					fmt.Fprintf(os.Stderr, " %d/%d\n", i+1, len(results))
				}
			}
			fmt.Fprintln(os.Stderr)
		}

		total := passed + failed + errors
		accuracy := float64(0)
		avgLat := float64(0)
		if total > 0 {
			accuracy = float64(passed) / float64(total) * 100
			avgLat = float64(catLatency) / float64(total)
		}

		fmt.Fprintf(os.Stderr, "%s: %d/%d (%.1f%%) avg %.0fms\n", cat.name, passed, total, accuracy, avgLat)
		allResults = append(allResults, results...)
	}

	// Build summary.
	var overallPassed, overallTotal, overallErrors int
	var overallLatency int64
	for _, r := range allResults {
		overallTotal++
		overallLatency += r.DurationMs
		if r.Error != "" {
			overallErrors++
		} else if r.Pass {
			overallPassed++
		}
	}

	overallAcc := float64(0)
	overallAvg := float64(0)
	if overallTotal > 0 {
		overallAcc = float64(overallPassed) / float64(overallTotal) * 100
		overallAvg = float64(overallLatency) / float64(overallTotal)
	}

	fmt.Fprintf(os.Stderr, "\n=== BFCL Overall ===\n")
	fmt.Fprintf(os.Stderr, "Model:    %s\n", *model)
	fmt.Fprintf(os.Stderr, "Provider: %s\n", *provider)
	fmt.Fprintf(os.Stderr, "Workers:  %d\n", *workers)
	fmt.Fprintf(os.Stderr, "Total:    %d\n", overallTotal)
	fmt.Fprintf(os.Stderr, "Passed:   %d\n", overallPassed)
	fmt.Fprintf(os.Stderr, "Accuracy: %.1f%%\n", overallAcc)
	fmt.Fprintf(os.Stderr, "Avg lat:  %.0fms\n", overallAvg)

	// Print failures to stderr.
	failCount := 0
	for _, r := range allResults {
		if !r.Pass {
			failCount++
		}
	}
	if failCount > 0 {
		fmt.Fprintf(os.Stderr, "\n--- Failures (%d) ---\n", failCount)
		for _, r := range allResults {
			if !r.Pass {
				fmt.Fprintln(os.Stderr, bfcl.FormatResult(r))
			}
		}
	}

	// Structured JSON to stdout.
	report := bfcl.RunReport{
		RunID:    runID,
		Model:    *model,
		Provider: *provider,
		Parameters: map[string]any{
			"workers":   *workers,
			"use_loop":  *useLoop,
			"stream":    *useStream,
			"max_tokens": 1024,
			"temperature": 0,
		},
		Results: allResults,
		Summary: bfcl.ReportSummary{
			Total:      overallTotal,
			Passed:     overallPassed,
			Failed:     overallTotal - overallPassed - overallErrors,
			Errors:     overallErrors,
			Accuracy:   overallAcc,
			AvgLatency: overallAvg,
		},
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		fmt.Fprintf(os.Stderr, "encode report: %v\n", err)
		os.Exit(1)
	}
}

func runCategory(client *openai.Client, model string, cat bfcl.Category, cases []bfcl.TestCase, numWorkers int, useLoop bool, useStream bool) []bfcl.Result {
	results := make([]bfcl.Result, len(cases))
	jobs := make(chan job, len(cases))

	var runner *bfcl.Runner
	if useLoop {
		runner = bfcl.NewRunner(client, model)
	}

	var done atomic.Int64
	total := int64(len(cases))
	start := time.Now()

	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				var r bfcl.Result
				if runner != nil {
					r = runWithLoop(runner, j.tc, j.cat)
				} else {
					r = runDirect(client, model, j.tc, j.cat, useStream)
				}
				r.Category = j.cat
				results[j.index] = r
				n := done.Add(1)
				if n%25 == 0 {
					elapsed := time.Since(start).Seconds()
					rate := float64(n) / elapsed
					remaining := float64(total-n) / rate
					fmt.Fprintf(os.Stderr, "  %d/%d (%.0f/s, ~%.0fs remaining)\n", n, total, rate, remaining)
				}
			}
		}()
	}

	for i, tc := range cases {
		if len(tc.Question) == 0 || len(tc.Question[0]) == 0 {
			continue
		}
		jobs <- job{index: i, tc: tc, cat: cat}
	}
	close(jobs)
	wg.Wait()

	return results
}

func runDirect(client *openai.Client, model string, tc bfcl.TestCase, cat bfcl.Category, stream bool) bfcl.Result {
	think := false
	msgs := bfcl.ToMessages(tc.Question[0])
	tools := bfcl.ToTools(tc.Functions)

	req := &loop.Request{
		Model:    model,
		Messages: msgs,
		Tools:    tools,
		Stream:   stream,
		Think:    &think,
		Options:  map[string]any{"max_tokens": 1024, "temperature": float64(0)},
	}

	start := time.Now()
	var resp loop.Response
	err := client.Chat(context.Background(), req, func(r loop.Response) error {
		// When streaming, accumulate content and tool calls across chunks.
		if stream {
			resp.Content += r.Content
			resp.Thinking += r.Thinking
			resp.ToolCalls = append(resp.ToolCalls, r.ToolCalls...)
			if r.Done {
				resp.Done = true
			}
			return nil
		}
		resp = r
		return nil
	})

	r := bfcl.Result{
		ID:         tc.ID,
		DurationMs: bfcl.Elapsed(start),
		Expected:   bfcl.FormatExpected(tc.Truth),
		Got:        bfcl.FormatGot(resp.ToolCalls),
	}

	if cat == bfcl.Irrelevance {
		r.Expected = "(no call)"
		if len(resp.ToolCalls) == 0 {
			r.Got = "(no call)"
		}
	}

	if err != nil {
		r.Error = err.Error()
	} else if bfcl.Grade(resp.ToolCalls, tc.Truth, cat) {
		r.Pass = true
	}

	return r
}

func runWithLoop(runner *bfcl.Runner, tc bfcl.TestCase, cat bfcl.Category) bfcl.Result {
	start := time.Now()
	calls, err := runner.Run(context.Background(), tc, cat)

	r := bfcl.Result{
		ID:         tc.ID,
		DurationMs: bfcl.Elapsed(start),
		Expected:   bfcl.FormatExpected(tc.Truth),
		Got:        bfcl.FormatGot(calls),
	}

	if cat == bfcl.Irrelevance {
		r.Expected = "(no call)"
		if len(calls) == 0 {
			r.Got = "(no call)"
		}
	}

	if err != nil {
		r.Error = err.Error()
	} else if bfcl.Grade(calls, tc.Truth, cat) {
		r.Pass = true
	}

	return r
}

func loadCategory(qPath, aPath string) ([]bfcl.TestCase, error) {
	if aPath == "" {
		return bfcl.LoadTestCases(qPath, os.DevNull)
	}
	return bfcl.LoadTestCases(qPath, aPath)
}

func newRunID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("bfcl-%x", b)
}

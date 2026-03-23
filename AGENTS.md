# axon-eval

Evaluation framework for running scenario plans against a live service cluster.

## Build & Test

```bash
go test ./...
go vet ./...
```

## Key Files

- `eval.go` — core evaluation engine and EvalScenario/EvalReport types
- `client.go` — HTTP client for interacting with services under test
- `plan.go` — YAML test plan loading and validation
- `run.go` — test run execution and ScenarioResult types
- `grade.go` — rubric grading with criterion evaluation
- `judge.go` — LLM-based judge interface and OllamaJudge implementation
- `verify.go` — post-run analytics verification
- `doc.go` — package documentation
- `bfcl/` — Berkeley Function Calling Leaderboard evaluation harness

@AGENTS.md

## Conventions
- Plans are YAML files defining scenarios with expected outcomes
- Results use structured types (EvalReport, ScenarioResult)
- Grading uses rubric-based criteria via LLM judge (OllamaJudge)
- bfcl/ is a separate evaluation suite — changes there don't affect the core engine

## Constraints
- Standalone tool — no other axon-* modules depend on this, do not export shared types
- Evaluates against a running cluster — all tests are integration-level by nature
- Do not add axon (HTTP toolkit) as a dependency — this is a test tool, not a service

## Testing
- `go test ./...` and `go vet ./...`
- Tests that hit a live cluster need the cluster running — skip gracefully if unavailable

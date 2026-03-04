# axon-test

An evaluation framework for running scenario plans against a live service cluster.

Supports YAML-defined test plans with assertions on HTTP responses and LLM-generated content.

## Install

```
go get github.com/benaskins/axon-test@latest
```

Requires Go 1.25+.

## Usage

```go
plan, _ := test.LoadPlan("plans/smoke.yaml")
client := test.NewClient(test.Config{BaseURL: clusterURL})

for _, scenario := range plan.Scenarios {
    result, _ := client.RunScenario(ctx, scenario)
    grade := test.GradeScenario(result, judge)
}
```

### Key types

- `Plan`, `PlanScenario` — YAML test plan structure
- `Scenario`, `Message` — scenario definition with messages and expectations
- `Run`, `ScenarioResult` — execution results
- `Criterion`, `ToolExpect` — assertion types
- `Judge` — LLM-based evaluation interface
- `OllamaJudge` — Ollama-backed judge implementation
- `Client` — HTTP client for running scenarios against a cluster

## License

Apache 2.0 — see [LICENSE](LICENSE).

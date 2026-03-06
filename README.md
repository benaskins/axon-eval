# axon-eval

An evaluation framework for running scenario plans against a live service cluster. Part of [lamina](https://github.com/benaskins/lamina) — each axon package can be used independently.

Supports YAML-defined test plans with assertions on HTTP responses and LLM-generated content.

## Install

```
go get github.com/benaskins/axon-eval@latest
```

Requires Go 1.25+.

## Usage

```go
plan, _ := eval.LoadPlan("plans/smoke.yaml")
client := eval.NewClient(eval.Config{BaseURL: clusterURL})

for _, scenario := range plan.Scenarios {
    result, _ := client.RunScenario(ctx, scenario)
    grade := eval.GradeScenario(result, judge)
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

MIT — see [LICENSE](LICENSE).

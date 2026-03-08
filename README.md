# axon-eval

> Standalone tool · Part of the [lamina](https://github.com/benaskins/lamina-mono) workspace

Evaluation framework for scenario testing against live services. Define test plans in YAML with rubric criteria — response content checks, duration limits, tool-use expectations, and LLM-judged quality — then run them against a service cluster and collect graded results.

## Getting started

```
go get github.com/benaskins/axon-eval@latest
```

Requires Go 1.25+.

Create a plan file (see [`example/smoke.yaml`](example/smoke.yaml)):

```yaml
name: smoke test
scenarios:
  - name: greeting
    message: "Hello, how are you?"
    ideal_response: "A warm, friendly greeting."
    max_duration_ms: 5000
    rubric:
      - type: min_length
        value: "20"
      - type: llm_judge
        criterion: "Response is warm and conversational in tone"
```

Run it from Go:

```go
plan, _ := eval.LoadPlan("example/smoke.yaml")

client, _ := eval.NewClient(eval.Config{
    AuthURL:      "https://auth.studio.internal",
    ChatURL:      "https://chat.studio.internal",
    AnalyticsURL: "https://look.studio.internal",
})

run, _ := client.Run("smoke test", []eval.Scenario{
    eval.Conversation("greeting", []eval.Message{
        {Role: "user", Content: plan.Scenarios[0].Message},
    }),
})

grade := eval.GradeScenario(plan.Scenarios[0], run.Responses[0].Responses[0], judge)
```

Or use the `lamina eval` command from the workspace:

```bash
lamina eval plans/smoke.yaml
```

## Key types

- **`Plan`**, **`PlanScenario`** — YAML test plan structure with scenarios, rubrics, and tool expectations
- **`Criterion`**, **`ToolExpect`** — assertion types: `contains`, `not_contains`, `min_length`, `max_length`, `llm_judge`
- **`Client`**, **`Config`** — HTTP client for running scenarios against auth, chat, and analytics services
- **`Run`**, **`ScenarioResult`**, **`ChatResult`** — execution results with response text, duration, and tools used
- **`Judge`**, **`OllamaJudge`** — LLM-based grading interface and Ollama-backed implementation
- **`ScenarioGrade`**, **`CriterionResult`** — graded results per scenario and per criterion
- **`EvalScenario`**, **`EvalReport`** — programmatic evaluation with custom `CheckFunc` assertions

## License

MIT — see [LICENSE](LICENSE).

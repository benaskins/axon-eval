# Eval Explorer - Design Document
# 2026-03-30

## What

A chat-first web application for exploring eval data. An LLM agent queries ClickHouse, reasons about the results, and renders interactive visualisations inline in the conversation. Its specialty is visualising data. It approaches eval results with curiosity, noticing patterns, flagging missing data, suggesting what to capture next.

Name: TBD (working name: eval-explorer)

## Use Cases

### Autotune exploration
"Show me before and after for the last autotune run."
Agent queries two runs, renders a Difference chart, explains which adaptations were kept and why.

### BFCL run comparison
"Compare the last 3 runs. What changed?"
Agent identifies differences in model, parameters, system prompt, tooling version. Renders a heatmap of accuracy by model x category.

### Failure analysis
"What's failing in parallel_multiple?"
Agent queries per-case results, identifies patterns, renders latency distributions and error breakdowns.

### Data quality
"What should I be capturing that I'm not?"
Agent notices empty prompt_hash fields, missing provider metadata, suggests enrichments.

## Architecture

### Service composition

Lives inside axon-eval, alongside the schemas, eval tooling, and ingest CLI. The explorer is how you look at what axon-eval produces.

```
axon-eval/
├── cmd/
│   ├── eval-ingest/          # already exists
│   └── explorer/             # new: service entry point
│       └── main.go
├── explorer/                 # new: agent + embedded frontend
│   ├── agent.go              # agent definition + tool registry
│   ├── tools.go              # server-side tools (ClickHouse queries)
│   ├── server.go             # HTTP server, /api/chat SSE endpoint
│   ├── embed.go              # //go:embed static
│   ├── web/                  # SvelteKit source
│   │   └── src/
│   │       ├── routes/       # chat interface
│   │       └── lib/
│   │           ├── dispatcher.js    # tool call dispatcher
│   │           └── charts/          # Observable Plot renderers
│   └── static/               # embedded build output
├── schemas.go                # already exists
├── bfcl/                     # already exists
└── docs/plans/
```

Dependencies:
- axon (HTTP lifecycle, middleware)
- axon-loop (conversation loop)
- axon-talk (LLM provider adapters)
- axon-tool (tool definitions)
- axon-look (ClickHouse HTTP client only)

### Tool architecture

Tools split into two execution contexts:

**Server-side tools** (executed by the agent runtime, results returned to LLM):
- `query_eval_runs` - list BFCL runs with filters (model, date range, provider)
- `query_run_detail` - per-case results for a run
- `query_compare` - accuracy by model x category for recent runs
- `query_raw` - freeform ClickHouse SQL against eval tables (guardrailed to SELECT only)

**Client-side tools** (dispatched to frontend via SSE, rendered in conversation):
- `render_bar_chart` - data + x/y mappings + optional grouping
- `render_line_chart` - data + x/y + optional series
- `render_heatmap` - data + x/y + color value
- `render_difference` - two series + labels (before/after)
- `render_box_plot` - data + category + value
- `render_table` - structured data as a formatted table
- `render_dot_plot` - scatter with x/y + optional size/color

### Message flow

```
User types question
  -> POST /api/chat (SSE stream)
  -> Agent reasons, calls server-side tool (e.g. query_eval_runs)
  -> Tool executes ClickHouse query, returns data to agent
  -> Agent reasons about data
  -> Agent calls client-side tool (e.g. render_bar_chart)
  -> SSE delivers tool_call event to frontend
  -> JS dispatcher matches tool name, calls Observable Plot renderer
  -> SVG chart rendered inline in conversation
  -> Agent continues with text explanation
```

Client-side tools are terminal: the agent does not see the rendered output. It trusts the tool call was correct. Feedback loop (agent sees screenshot) is a future enhancement.

### Frontend

SvelteKit static SPA, embedded via `//go:embed`. Single route: a chat interface.

**Tool dispatcher** (`lib/dispatcher.js`):
- Receives tool_call events from SSE stream
- Matches tool name to renderer function
- Server-side tools: no action (already executed)
- Client-side tools: calls renderer, inserts output into conversation

**Chart renderers** (`lib/charts/`):
- Each renderer takes structured args (data array + column mappings)
- Returns an SVG DOM element via Observable Plot
- Renderers: bar, line, heatmap, difference, box, table, dot

**Styling**: Dark theme consistent with existing lamina aesthetic.

### LLM selection

Start with local Qwen3.5-122B via the OpenAI adapter (vLLM-MLX on port 8091). Write a BFCL-style eval that tests the agent's ability to:
1. Select the right query tool for a question
2. Select the right visualisation for the data
3. Produce valid tool call arguments

If local model scores below threshold, fall back to Anthropic via axon-talk.

## Agent system prompt (sketch)

```
You are an eval data explorer. You help users understand their evaluation
results through visualisation and analysis.

You have access to ClickHouse tables containing BFCL benchmark results
and autotune experiment data. When a user asks a question:

1. Query the relevant data using your tools
2. Visualise the results, prefer charts over text for comparisons
3. Explain what you see, be curious, notice patterns
4. Point out missing data that would be useful to capture

Always visualise when comparing. Use tables for small datasets (<10 rows),
charts for everything else. Pick the chart type that best shows the
relationship the user is asking about.
```

## Implementation Steps

### Phase 1: Scaffold + chat loop
1. Create axon-eval-explorer repo with luthier manifest
2. Wire axon + axon-loop + axon-talk with OpenAI adapter
3. Basic chat endpoint with SSE streaming
4. Minimal SvelteKit frontend: chat input + message stream

### Phase 2: Server-side tools
5. Implement query tools against ClickHouse (eval_bfcl table)
6. Register tools with agent
7. Test tool selection with local model

### Phase 3: Client-side visualisation
8. Add Observable Plot to frontend
9. Implement chart renderers (bar, line, heatmap, table)
10. Build JS tool dispatcher
11. Wire SSE tool_call events to dispatcher

### Phase 4: Agent personality + polish
12. Tune system prompt for curiosity and data quality suggestions
13. Add remaining chart types (difference, box, dot)
14. Write model selection eval
15. Deploy under aurelia

## Repos Touched

| Repo | Changes |
|------|---------|
| axon-eval | explorer/ package, cmd/explorer/ entry point, SvelteKit frontend |
| hestia-core-infrastructure | Aurelia service spec, binary |

## Decisions

- **Hostname**: `explorer.hestia.internal`
- **No memory** (axon-memo): not needed for v1
- **No query_raw**: structured query tools cover the use cases, add freeform SQL later if needed
- **Import axon-look**: use ClickHouse HTTP client, not the full analytics server
- **Lives in axon-eval**: the explorer is how you look at what axon-eval produces

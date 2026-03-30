# Eval Pipeline Design

Date: 2026-03-29
Status: Design agreed, not yet implemented

## Problem

Eval results (BFCL, luthier-eval) are JSON files on disk with no queryable history. Cannot trend scores across models, compare runs, or correlate results with prompt changes. Model parameters and system prompts are not captured alongside results.

More broadly, axon-look hardcodes event schemas and reimplements "store an event" outside of axon-fact. Every typed event in the system (eval results, chat messages, memory extractions) should flow through the same pipeline with pluggable sinks, not bespoke per-domain ingestion.

## Target Architecture

### Fact Pipeline

An eval result is a Fact. So is a chat message, a tool invocation, a memory extraction. All facts flow through the same pipeline with pluggable sinks:

```
Fact (typed event with schema)
    |
    +---> EventStore (append, replay) - postgres
    +---> Publisher (fan-out) - nats
    +---> Projector (materialize) - clickhouse
```

The composition root (axon-eval, axon-chat, etc.) defines its fact types, registers their schemas, and wires up which sinks each fact flows to.

### Ownership

```
axon-fact (contract + orchestration)
    Schema: Go struct metadata -> field name, type, cardinality, ordering
    EventStore interface: Append, Replay
    Publisher interface: Publish
    Projector interface: Project
    Pipeline: wires facts to sinks
        |
        +--- axon-eval defines: eval_bfcl, eval_luthier fact types
        +--- axon-memo defines: memory_extracted, consolidation_completed
        +--- axon-chat defines: message, tool_invocation, conversation
        |
        v
axon-look: Projector implementation for ClickHouse
    Given a schema, creates tables and inserts rows
    No domain knowledge, no hardcoded event types
    Query endpoints are schema-driven

axon-nats: Publisher implementation
    Given a fact, publishes to a subject derived from the type

postgres EventStore: append-only event log
    Schema-agnostic, stores (id, type, timestamp, data jsonb)
```

### Composition Root Example (axon-eval)

```go
bfclSchema := eval.BFCLResultSchema()
luthierSchema := eval.LuthierResultSchema()

store := postgres.NewEventStore(db)
projector := look.NewProjector(ch)
publisher := nats.NewPublisher(nc)

pipeline := fact.NewPipeline(
    fact.WithStore(store),
    fact.WithProjector(projector, bfclSchema, luthierSchema),
    fact.WithPublisher(publisher),
)

// Recording a result flows to all sinks
pipeline.Record(ctx, eval.BFCLResult{...})
```

### Eval-Specific Components

```
bfcl-run / luthier-eval
        | (JSON to stdout, enriched with prompt + parameters)
        v
   eval-ingest (stdin)
        |
        +---> postgres: prompt store (upsert by content hash)
        |
        +---> fact.Pipeline.Record()
                |
                +---> EventStore (postgres event log)
                +---> Projector (clickhouse: eval_bfcl / eval_luthier)
                +---> Publisher (nats, optional)
```

### Prompt Store (postgres)

Prompts are versioned artifacts, not event data. Content-addressed storage.

```sql
CREATE TABLE eval_prompts (
    hash       TEXT PRIMARY KEY,
    name       TEXT NOT NULL,       -- e.g. "luthier/analysis", "bfcl/system"
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_eval_prompts_name ON eval_prompts(name);
```

### ClickHouse Tables

Schema defined by axon-eval fact types, tables created by the axon-look Projector from schema metadata.

Fact type `eval_bfcl`, one row per test case:

| Column | Type | Notes |
|--------|------|-------|
| timestamp | DateTime64(3) | |
| run_id | String | Groups all cases in one invocation |
| model | LowCardinality(String) | `qwen3.5-397b`, `claude-sonnet-4-6` |
| provider | LowCardinality(String) | `local`, `cloudflare`, `anthropic` |
| category | LowCardinality(String) | `simple`, `multiple`, `parallel`, `parallel_multiple`, `irrelevance` |
| case_id | String | `simple_0`, `parallel_12` |
| pass | Bool | |
| error | String | Empty if no error |
| expected | String | Formatted expected call |
| got | String | Formatted actual call |
| duration_ms | UInt32 | Per-case latency |
| prompt_hash | String | FK-by-convention to postgres |
| parameters | String | JSON: `{"temperature": 0, "max_tokens": 1024}` |

Ordered by `(model, category, timestamp)`.

Fact type `eval_luthier`, one row per eval run:

| Column | Type | Notes |
|--------|------|-------|
| timestamp | DateTime64(3) | |
| run_id | String | |
| model | LowCardinality(String) | |
| provider | LowCardinality(String) | |
| runs | UInt16 | Number of analysis runs |
| module_score | Float32 | |
| boundary_score | Float32 | |
| gap_score | Float32 | |
| plan_step_score | Float32 | |
| overall_score | Float32 | |
| total_duration_ms | UInt32 | |
| mean_duration_ms | UInt32 | |
| prompt_hash | String | FK-by-convention to postgres |
| parameters | String | JSON |

Ordered by `(model, timestamp)`.

### Cross-Store Queries

ClickHouse PostgreSQL table engine for joining eval results with prompt content:

```sql
CREATE TABLE prompts_remote
ENGINE = PostgreSQL('infra-postgres:5432', 'eval', 'eval_prompts', 'user', 'pass');

SELECT e.model, e.overall_score, p.name, p.content
FROM eval_luthier e
JOIN prompts_remote p ON e.prompt_hash = p.hash
WHERE e.model = 'qwen3.5-397b'
ORDER BY e.timestamp DESC;
```

## Implementation Steps

### Phase 1: axon-fact Pipeline (prerequisite, benefits all domains)

**Step 1: Schema metadata in axon-fact**
Add `Schema` type to axon-fact: a struct carrying field definitions (name, type enum, cardinality hint, default). Go struct tags or explicit registration. No storage dependencies. The schema is the source of truth for what a fact looks like.

Type enum covers: String, LowCardinalityString, Bool, UInt16, UInt32, Float32, Float64, DateTime64, JSON (string holding JSON).

**Step 2: Projector interface in axon-fact**
```go
type Projector interface {
    EnsureSchema(ctx context.Context, schemas ...Schema) error
    Project(ctx context.Context, facts ...Fact) error
}
```

**Step 3: Pipeline in axon-fact**
`Pipeline` wires a fact to multiple sinks via functional options. `Record()` fans out to all configured sinks. Existing `EventStore` and `Publisher` interfaces are already defined; Pipeline composes them with the new Projector.

**Step 4: ClickHouse Projector in axon-look**
Refactor axon-look to implement `fact.Projector`. `EnsureSchema` generates CREATE TABLE DDL from schema metadata. `Project` does generic parameterized inserts. The hardcoded switch in `insertQuery()` is replaced by schema-driven logic. Existing event types (message, tool_invocation, etc.) migrate to fact.Schema registrations.

### Phase 2: Eval Pipeline

**Step 5: Eval fact types in axon-eval**
Implement `fact.Schema` for `eval_bfcl` and `eval_luthier`. These are Go structs with schema metadata describing the ClickHouse columns above.

**Step 6: Enrich bfcl-run output**
Add `model`, `provider`, `parameters`, `system_prompt`, and per-case results to the JSON output. Text summary moves to stderr, structured JSON to stdout.

**Step 7: Enrich luthier-eval output**
Add `model`, `provider`, `parameters`, `system_prompt` fields to the Report struct.

**Step 8: Prompt store migration**
Add `eval_prompts` table to postgres.

**Step 9: eval-ingest CLI**
Standalone Go binary in axon-eval `cmd/eval-ingest/`. Reads stdin, detects format, hashes prompt, upserts to postgres, records facts via Pipeline (which fans out to EventStore + Projector).

**Step 10: ClickHouse-Postgres bridge**
Create the `prompts_remote` PostgreSQL engine table in ClickHouse for cross-store joins.

**Step 11: Query endpoints**
Add axon-look endpoints for eval dashboards:
- `GET /api/evals/bfcl?model=X` - accuracy by category over time
- `GET /api/evals/luthier?model=X` - consistency scores over time
- `GET /api/evals/compare?models=X,Y` - side-by-side comparison

### Phase 3: Migrate Existing Domains (optional, incremental)

Migrate axon-memo, axon-chat event types from hardcoded axon-look schemas to axon-fact Schema registrations. Each domain owns its fact types; axon-look becomes purely generic.

## Repos Touched

| Repo | Phase | Changes |
|------|-------|---------|
| axon-fact | 1 | Schema type, Projector interface, Pipeline |
| axon-look | 1+2 | ClickHouse Projector implementation, generic ingest, query endpoints |
| axon-eval | 2 | Fact type schemas, enrich bfcl-run/luthier-eval output, eval-ingest CLI |
| luthier | 2 | Enrich Report with prompt + parameters |
| hestia-core-infrastructure | 2 | eval_prompts postgres migration, ClickHouse-Postgres bridge |
| axon-memo | 3 | Migrate to fact.Schema |
| axon-chat | 3 | Migrate to fact.Schema |

## Open Questions

- Should eval-ingest also write the JSON file to disk as a backup, or is that the caller's responsibility (`bfcl-run ... | tee results.json | eval-ingest`)?
- Do we need auth on the prompt store writes, or is postgres network-level access sufficient?
- Should Pipeline.Record() be synchronous (wait for all sinks) or async with error channels? Eval wants synchronous (know it was stored). Chat may want async (don't block the response).
- Should the query endpoints in axon-look also become schema-driven, or are they bespoke per use case? Aggregation queries (accuracy over time, score trends) are hard to generalize.

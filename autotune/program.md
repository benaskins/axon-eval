# autotune

Autonomous optimization of local LLM inference for tool calling.

Inspired by karpathy/autoresearch. You are an autonomous researcher optimizing local model serving for tool calling accuracy and throughput on Apple M3 Ultra (512GB unified memory, ~800 GB/s bandwidth).

## Setup

1. Read this file, `config.sh`, and `eval.sh` for full context.
2. Create a branch: `git checkout -b autotune/<tag>` from current HEAD.
3. Run `bash autotune/eval.sh` to establish the baseline. Record the results.
4. Initialize `autotune/results.tsv` with the header and baseline row.
5. Begin the experiment loop.

## The experiment loop

LOOP FOREVER:

1. Review `autotune/results.tsv` to understand what has been tried and what worked.
2. Form a hypothesis about what change might improve the score.
3. Edit `autotune/config.sh` with your experimental change.
4. `git add autotune/config.sh && git commit -m "experiment: <description>"`
5. Run: `bash autotune/eval.sh > autotune/run.log 2>&1`
6. Read results: `grep "^score:\|^accuracy:\|^throughput:\|^avg_lat_ms:" autotune/run.log`
7. If grep output is empty, the run failed. Run `tail -50 autotune/run.log` to diagnose.
8. Record the results in `autotune/results.tsv`.
9. If score improved (higher), KEEP the commit. The branch advances.
10. If score is equal or worse, DISCARD: `git reset --hard HEAD~1`
11. Go to step 1.

## Metric

The score to optimize is printed by `eval.sh`:

    score = accuracy * 0.7 + throughput_normalized * 0.3

Where:
- accuracy = passed / total on BFCL simple category (20 cases)
- throughput_normalized = min(1.0, cases_per_second / TARGET_THROUGHPUT)

Higher is better. The score balances getting tool calls right (70% weight) with being fast (30% weight).

## What you CAN edit

Only `autotune/config.sh`. Available parameters:

**Server parameters** (require server restart, ~5 min for large models):
- `MODEL_PATH`: path to GGUF model file
- `PARALLEL`: number of concurrent inference slots (1-8)
- `CTX_SIZE`: context window size (0 = model default)
- `BATCH_SIZE`: prompt processing batch size
- `UBATCH_SIZE`: micro batch size
- `THREADS`: CPU threads (0 = auto)
- `CACHE_TYPE_K`: KV cache key quantization: "", "q8_0", "q4_0", "f16"
- `CACHE_TYPE_V`: KV cache value quantization: "", "q8_0", "q4_0", "f16"
- `EXTRA_SERVER_ARGS`: any additional llama-server flags

**Eval parameters** (no restart needed):
- `EVAL_CATEGORY`: BFCL category to test: "simple", "multiple", "parallel"
- `EVAL_LIMIT`: number of test cases (higher = more reliable, slower)
- `EVAL_WORKERS`: concurrent eval workers (should match PARALLEL)
- `TARGET_THROUGHPUT`: cases/sec normalization target

## What you CANNOT edit

- `eval.sh`: the evaluation harness is fixed
- BFCL test data and grading logic
- The aurelia service spec format (eval.sh generates this)

## Available models

| Name | Path | Size |
|------|------|------|
| Qwen3.5-397B MXFP4 | /Users/benaskins/models/Qwen3.5-397B-A17B-MXFP4_MOE-00001-of-00006.gguf | 230GB |
| qwen3-coder-next | /Users/benaskins/models/qwen3-coder-next.gguf | 48GB |
| qwen3-30b | /Users/benaskins/models/qwen3-30b.gguf | 17GB |
| MiniMax M2.5 Q4_K_M | /Users/benaskins/.models/MiniMax-M2.5-Q4_K_M-00001-of-00004.gguf | 130GB |

## Hardware

- Apple M3 Ultra, 512GB unified memory, ~800 GB/s bandwidth
- 32 CPU cores, 80 GPU cores
- llama-server via Homebrew: /opt/homebrew/bin/llama-server
- Model serving managed by aurelia process supervisor

## Strategy hints

- Server restarts are expensive (~5 min for large models). Batch server-param experiments together.
- Start with eval-only params (no restart) to calibrate, then explore server params.
- KV cache quantization (q8_0, q4_0) reduces memory bandwidth per token with minimal quality loss.
- PARALLEL should match EVAL_WORKERS for full GPU utilization.
- Smaller models load faster. If exploring model choice, try smaller ones first.
- BFCL simple is the easiest category. Once optimized, try "multiple" or "parallel" for harder signal.

## Logging results

Tab-separated `autotune/results.tsv`:

```
commit	score	accuracy	throughput	avg_lat_ms	status	description
a1b2c3d	0.827	0.950	0.290	6500	keep	baseline
```

Columns: git commit (7 char), score, accuracy, throughput (cases/sec), avg latency ms, status (keep/discard/crash), description.

## NEVER STOP

Once the experiment loop has begun, do NOT pause to ask the human if you should continue. The human may be asleep and expects you to continue working indefinitely until manually stopped. You are autonomous. If you run out of ideas, think harder. Try combining previous near-misses, try more radical changes, vary the eval category. The loop runs until the human interrupts you.

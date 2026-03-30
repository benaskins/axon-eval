# autotune (MLX)

Autonomous optimization of local LLM inference via vllm-mlx for tool calling on Apple M3 Ultra.

## Setup

1. Read this file, `mlx-config.sh`, and `mlx-eval.sh` for full context.
2. Create a branch: `git checkout -b autotune/mlx-<tag>` from current HEAD.
3. Run `bash autotune/mlx-eval.sh` to establish the baseline. Record the results.
4. Initialize `autotune/mlx-results.tsv` with the header and baseline row.
5. Begin the experiment loop.

## The experiment loop

LOOP FOREVER:

1. Review `autotune/mlx-results.tsv` to understand what has been tried and what worked.
2. Form a hypothesis about what change might improve the score.
3. Edit `autotune/mlx-config.sh` with your experimental change.
4. `git add autotune/mlx-config.sh && git commit -m "experiment: <description>"`
5. Run: `bash autotune/mlx-eval.sh > autotune/mlx-run.log 2>&1`
6. Read results: `grep "^score:\|^accuracy:\|^throughput:\|^avg_lat_ms:" autotune/mlx-run.log`
7. If grep output is empty, the run failed. Run `tail -50 autotune/mlx-run.log` to diagnose.
8. Record the results in `autotune/mlx-results.tsv`.
9. If score improved (higher), KEEP the commit. The branch advances.
10. If score is equal or worse, DISCARD: `git reset --hard HEAD~1`
11. Go to step 1.

## Metric

    score = accuracy * 0.7 + throughput_normalized * 0.3

Higher is better. 15 test cases (3 per BFCL category). Each experiment takes ~2-3 minutes.

## What you CAN edit

Only `autotune/mlx-config.sh`. Available parameters:

**Model**: try different models from the available list below.

**Server parameters**:
- `MAX_NUM_SEQS`: concurrent sequences (1-8)
- `PREFILL_BATCH_SIZE`: prompt processing batch size
- `COMPLETION_BATCH_SIZE`: token generation batch size
- `STREAM_INTERVAL`: tokens to batch before streaming (1=smooth, higher=throughput)
- `MAX_TOKENS`: max generation tokens
- `CONTINUOUS_BATCHING`: true/false (enables batched scheduling for concurrent users)
- `PREFIX_CACHE`: true/false (caches common prompt prefixes)
- `CACHE_MEMORY_PERCENT`: fraction of RAM for cache (default 0.20)
- `EXTRA_ARGS`: any additional vllm-mlx flags

**Eval parameters**:
- `EVAL_LIMIT`: cases per category
- `EVAL_WORKERS`: concurrent eval workers (should not exceed MAX_NUM_SEQS)
- `TARGET_THROUGHPUT`: normalization target for throughput score

## What you CANNOT edit

- `mlx-eval.sh`: the evaluation harness is fixed
- BFCL test data and grading logic

## Available models

| Name | Path | Size | Active params |
|------|------|------|---------------|
| Qwen3.5-122B-A10B 5bit | /Users/benaskins/models/mlx/Qwen3.5-122B-A10B-5bit | 79GB | 10B |
| Qwen3.5-27B 8bit | /Users/benaskins/models/mlx/Qwen3.5-27B-8bit | 28GB | 27B (dense) |
| Qwen3.5-27B 4bit | /Users/benaskins/models/mlx/Qwen3.5-27B-4bit | 15GB | 27B (dense) |
| Qwen3-30B-A3B 8bit | /Users/benaskins/models/mlx/Qwen3-30B-A3B-8bit | 30GB | 3B |

## Hardware

- Apple M3 Ultra, 512GB unified memory, ~800 GB/s bandwidth
- 32 CPU cores, 80 GPU cores
- vllm-mlx installed via pipx

## Strategy hints

- Model swaps are cheap (~4-20s load time). Try different models early.
- The 122B-5bit had the best smoke test results (100% accuracy, 3.6s avg latency).
- Try increasing MAX_NUM_SEQS with matching EVAL_WORKERS for throughput.
- CONTINUOUS_BATCHING may help with concurrent requests but had issues previously.
- PREFIX_CACHE helps when the same system prompt is reused across requests (BFCL does this).
- Increasing CACHE_MEMORY_PERCENT may improve prefix cache hit rates.
- The 27B models are dense (all params active per token), 122B and 30B are MoE.

## Logging results

Tab-separated `autotune/mlx-results.tsv`:

```
commit	score	accuracy	throughput	avg_lat_ms	status	description
a1b2c3d	0.827	0.950	0.290	6500	keep	baseline 122B-5bit
```

## NEVER STOP

Once the experiment loop has begun, do NOT pause to ask the human if you should continue. The human may be asleep and expects you to continue working indefinitely until manually stopped. You are autonomous. If you run out of ideas, think harder. Try combining previous near-misses, try more radical changes, swap models. The loop runs until the human interrupts you.

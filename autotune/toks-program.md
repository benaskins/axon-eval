# autotune (tok/s)

Maximize generation throughput (tokens per second) for the Qwen3.5-122B-A10B-5bit model on Apple M3 Ultra via vllm-mlx.

## Setup

1. Read this file, `toks-config.sh`, and `toks-eval.sh` for full context.
2. Create a branch: `git checkout -b autotune/toks-<tag>` from current HEAD.
3. Run `bash autotune/toks-eval.sh` to establish the baseline. Record the results.
4. Initialize `autotune/toks-results.tsv` with the header and baseline row.
5. Begin the experiment loop.

## The experiment loop

LOOP FOREVER:

1. Review `autotune/toks-results.tsv` to understand what has been tried and what worked.
2. Form a hypothesis about what change might improve tok/s.
3. Edit `autotune/toks-config.sh` with your experimental change.
4. `git add autotune/toks-config.sh && git commit -m "experiment: <description>"`
5. Run: `bash autotune/toks-eval.sh > autotune/toks-run.log 2>&1`
6. Read results: `grep "^score:\|^mean_toks:\|^min_toks:\|^max_toks:" autotune/toks-run.log`
7. If grep output is empty, the run failed. Run `tail -50 autotune/toks-run.log` to diagnose.
8. Record the results in `autotune/toks-results.tsv`.
9. If score improved (higher mean tok/s), KEEP the commit. The branch advances.
10. If score is equal or worse, DISCARD: `git reset --hard HEAD~1`
11. Go to step 1.

## Metric

The score is **mean tok/s** across all requests. Higher is better. Baseline is ~22 tok/s.

## What you CAN edit

Only `autotune/toks-config.sh`. Available parameters:

**Concurrency and batching:**
- `MAX_NUM_SEQS`: concurrent inference slots (1-8). More slots = more throughput from batching, but each slot uses KV cache memory.
- `COMPLETION_BATCH_SIZE`: tokens generated per scheduling step. Higher may improve GPU utilization.
- `PREFILL_BATCH_SIZE`: prompt tokens processed per step.
- `CONTINUOUS_BATCHING`: true/false. Must be true for concurrent requests to overlap.
- `CONCURRENT`: number of concurrent eval requests (should match MAX_NUM_SEQS).

**KV cache:**
- `KV_CACHE_QUANTIZATION`: true/false. Quantize the KV cache to reduce memory per token.
- `KV_CACHE_QUANT_BITS`: 4 or 8. Lower = less memory = more bandwidth for weights, but potential quality loss.
- `KV_CACHE_GROUP_SIZE`: quantization group size (default 64).
- `CACHE_MEMORY_PERCENT`: fraction of RAM for prefix cache (default 0.20).

**Paged cache (experimental):**
- `PAGED_CACHE`: true/false. Paged KV cache for memory efficiency.
- `PAGED_CACHE_BLOCK_SIZE`: tokens per block (default 64).
- `MAX_CACHE_BLOCKS`: maximum blocks (default 1000).

**Eval parameters:**
- `NUM_REQUESTS`: total requests to send (more = more reliable measurement, slower).
- `MAX_TOKENS`: tokens generated per request (256 = medium length response).

## What you CANNOT edit

- `toks-eval.sh`: the evaluation harness is fixed.
- The model itself (stay on Qwen3.5-122B-A10B-5bit).

## Hardware

- Apple M3 Ultra, 512GB unified memory, ~800 GB/s bandwidth
- 32 CPU cores, 80 GPU cores
- Model uses ~79GB, leaving ~420GB free

## Strategy hints

- The M3 Ultra is memory-bandwidth-bound for generation. Anything that reduces bytes read per token helps.
- KV cache quantization (8-bit or 4-bit) reduces cache reads per token. Try it early.
- Increasing MAX_NUM_SEQS amortizes fixed overhead across more requests, but too many slots fragment the KV cache.
- COMPLETION_BATCH_SIZE controls how many tokens are generated before the scheduler checks for new work. Higher values may reduce scheduling overhead.
- PREFIX_CACHE helps when prompts share common prefixes, but uses memory. Try disabling it to free memory for larger batch sizes.
- Each experiment takes ~1-2 minutes (10 requests, 256 tokens each).

## Logging results

Tab-separated `autotune/toks-results.tsv`:

```
commit	score	mean_toks	min_toks	max_toks	requests	status	description
a1b2c3d	22.5	22.5	18.3	25.1	10	keep	baseline
```

## NEVER STOP

Once the experiment loop has begun, do NOT pause to ask the human if you should continue. You are autonomous. If you run out of ideas, think harder. The loop runs until the human interrupts you.

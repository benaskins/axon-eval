#!/bin/bash
# Fixed evaluation harness for vllm-mlx. DO NOT MODIFY.
# Sources mlx-config.sh, starts/restarts vllm-mlx, runs BFCL, outputs metrics.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EVAL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Defaults
MODEL_PATH=""
PORT=8091
MAX_NUM_SEQS=1
PREFILL_BATCH_SIZE=""
COMPLETION_BATCH_SIZE=""
STREAM_INTERVAL=""
MAX_TOKENS=4096
CONTINUOUS_BATCHING=false
PREFIX_CACHE=true
CACHE_MEMORY_PERCENT=""
EXTRA_ARGS=""
TOOL_CALL_PARSER="qwen3_coder"
REASONING_PARSER="qwen3"
EVAL_LIMIT=3
EVAL_WORKERS=1
TARGET_THROUGHPUT=0.15

# Source tunable config
source "$SCRIPT_DIR/mlx-config.sh"

# Build server command
SERVER_CMD="vllm-mlx serve ${MODEL_PATH} --port ${PORT} --max-num-seqs ${MAX_NUM_SEQS} --max-tokens ${MAX_TOKENS}"
SERVER_CMD="$SERVER_CMD --enable-auto-tool-choice --tool-call-parser ${TOOL_CALL_PARSER} --reasoning-parser ${REASONING_PARSER}"

[ "$CONTINUOUS_BATCHING" = true ] && SERVER_CMD="$SERVER_CMD --continuous-batching"
[ "$PREFIX_CACHE" = false ] && SERVER_CMD="$SERVER_CMD --disable-prefix-cache"
[ -n "$PREFILL_BATCH_SIZE" ] && SERVER_CMD="$SERVER_CMD --prefill-batch-size $PREFILL_BATCH_SIZE"
[ -n "$COMPLETION_BATCH_SIZE" ] && SERVER_CMD="$SERVER_CMD --completion-batch-size $COMPLETION_BATCH_SIZE"
[ -n "$STREAM_INTERVAL" ] && SERVER_CMD="$SERVER_CMD --stream-interval $STREAM_INTERVAL"
[ -n "$CACHE_MEMORY_PERCENT" ] && SERVER_CMD="$SERVER_CMD --cache-memory-percent $CACHE_MEMORY_PERCENT"
[ -n "$EXTRA_ARGS" ] && SERVER_CMD="$SERVER_CMD $EXTRA_ARGS"

# Kill any existing vllm-mlx on our port
existing=$(lsof -ti:${PORT} 2>/dev/null || true)
if [ -n "$existing" ]; then
    echo ">>> Killing existing server on port ${PORT}" >&2
    kill $existing 2>/dev/null || true
    sleep 3
fi

# Start server
echo ">>> Starting: $SERVER_CMD" >&2
eval "$SERVER_CMD" > /tmp/vllm-mlx-autotune.log 2>&1 &
VLLM_PID=$!

# Wait for healthy
echo ">>> Waiting for server..." >&2
timeout=120
start=$(date +%s)
while true; do
    if curl -s "http://localhost:${PORT}/v1/models" > /dev/null 2>&1; then
        elapsed=$(( $(date +%s) - start ))
        echo ">>> Server ready after ${elapsed}s" >&2
        break
    fi
    if ! kill -0 $VLLM_PID 2>/dev/null; then
        echo ">>> Server died during startup" >&2
        tail -10 /tmp/vllm-mlx-autotune.log >&2
        echo "accuracy:    0.000"
        echo "throughput:  0.000"
        echo "avg_lat_ms:  0"
        echo "wall_time_s: 0"
        echo "score:       0.000"
        exit 1
    fi
    elapsed=$(( $(date +%s) - start ))
    if [ "$elapsed" -gt "$timeout" ]; then
        echo ">>> TIMEOUT" >&2
        kill $VLLM_PID 2>/dev/null
        echo "accuracy:    0.000"
        echo "throughput:  0.000"
        echo "avg_lat_ms:  0"
        echo "wall_time_s: 0"
        echo "score:       0.000"
        exit 1
    fi
    sleep 2
done

# Run eval
echo ">>> Running BFCL (limit ${EVAL_LIMIT}, workers ${EVAL_WORKERS})..." >&2
eval_start=$(date +%s)

cd "$EVAL_DIR"
output=$(go run ./bfcl/cmd/bfcl-run/ \
    -dir bfcl/ \
    -provider local \
    -url "http://localhost:${PORT}" \
    -model "$MODEL_PATH" \
    -limit "$EVAL_LIMIT" \
    -workers "$EVAL_WORKERS" \
    2>&1)

eval_end=$(date +%s)
wall_time=$(( eval_end - eval_start ))

# Kill server after eval
kill $VLLM_PID 2>/dev/null || true

# Parse results
passed=$(echo "$output" | grep "^Passed:" | awk '{print $2}')
total=$(echo "$output" | grep "^Total:" | awk '{print $2}')
avg_lat=$(echo "$output" | grep "^Avg lat:" | awk '{print $2}' | sed 's/ms//')

if [ -z "$passed" ] || [ -z "$total" ] || [ "$total" = "0" ]; then
    echo ">>> Eval failed, no summary found" >&2
    echo "$output" | tail -20 >&2
    echo "accuracy:    0.000"
    echo "throughput:  0.000"
    echo "avg_lat_ms:  0"
    echo "wall_time_s: $wall_time"
    echo "score:       0.000"
    exit 1
fi

# Per-category breakdown to stderr
echo ">>> Per-category results:" >&2
echo "$output" | grep -E "^(simple|multiple|parallel_multiple|parallel|irrelevance):" >&2

# Calculate metrics
accuracy=$(awk "BEGIN {printf \"%.3f\", $passed / $total}")
cases_per_sec=$(awk "BEGIN {printf \"%.3f\", $total / $wall_time}")
throughput_norm=$(awk "BEGIN {t = $cases_per_sec / $TARGET_THROUGHPUT; if (t > 1.0) t = 1.0; printf \"%.3f\", t}")
score=$(awk "BEGIN {printf \"%.3f\", $accuracy * 0.7 + $throughput_norm * 0.3}")

echo "accuracy:    $accuracy"
echo "throughput:  $cases_per_sec"
echo "avg_lat_ms:  $avg_lat"
echo "wall_time_s: $wall_time"
echo "score:       $score"

#!/bin/bash
# Fixed tok/s evaluation harness. DO NOT MODIFY.
# Sources toks-config.sh, starts vllm-mlx, sends requests, measures tok/s.
#
# Output format:
#   mean_toks:   22.5
#   min_toks:    18.3
#   max_toks:    25.1
#   total_toks:  2560
#   wall_time_s: 45
#   score:       22.5

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Defaults
MODEL_PATH=""
PORT=8091
MAX_NUM_SEQS=4
PREFILL_BATCH_SIZE=""
COMPLETION_BATCH_SIZE=""
CONTINUOUS_BATCHING=true
PREFIX_CACHE=true
CACHE_MEMORY_PERCENT=""
KV_CACHE_QUANTIZATION=false
KV_CACHE_QUANT_BITS=""
KV_CACHE_GROUP_SIZE=""
PAGED_CACHE=false
PAGED_CACHE_BLOCK_SIZE=""
MAX_CACHE_BLOCKS=""
EXTRA_ARGS=""
NUM_REQUESTS=10
MAX_TOKENS=256
CONCURRENT=4

source "$SCRIPT_DIR/toks-config.sh"

# Build server command
SERVER_CMD="vllm-mlx serve ${MODEL_PATH} --port ${PORT} --max-num-seqs ${MAX_NUM_SEQS} --max-tokens ${MAX_TOKENS}"
SERVER_CMD="$SERVER_CMD --enable-auto-tool-choice --tool-call-parser qwen3_coder --reasoning-parser qwen3"

[ "$CONTINUOUS_BATCHING" = true ] && SERVER_CMD="$SERVER_CMD --continuous-batching"
[ "$PREFIX_CACHE" = false ] && SERVER_CMD="$SERVER_CMD --disable-prefix-cache"
[ -n "$PREFILL_BATCH_SIZE" ] && SERVER_CMD="$SERVER_CMD --prefill-batch-size $PREFILL_BATCH_SIZE"
[ -n "$COMPLETION_BATCH_SIZE" ] && SERVER_CMD="$SERVER_CMD --completion-batch-size $COMPLETION_BATCH_SIZE"
[ -n "$CACHE_MEMORY_PERCENT" ] && SERVER_CMD="$SERVER_CMD --cache-memory-percent $CACHE_MEMORY_PERCENT"
[ "$KV_CACHE_QUANTIZATION" = true ] && SERVER_CMD="$SERVER_CMD --kv-cache-quantization"
[ -n "$KV_CACHE_QUANT_BITS" ] && SERVER_CMD="$SERVER_CMD --kv-cache-quantization-bits $KV_CACHE_QUANT_BITS"
[ -n "$KV_CACHE_GROUP_SIZE" ] && SERVER_CMD="$SERVER_CMD --kv-cache-quantization-group-size $KV_CACHE_GROUP_SIZE"
[ "$PAGED_CACHE" = true ] && SERVER_CMD="$SERVER_CMD --use-paged-cache"
[ -n "$PAGED_CACHE_BLOCK_SIZE" ] && SERVER_CMD="$SERVER_CMD --paged-cache-block-size $PAGED_CACHE_BLOCK_SIZE"
[ -n "$MAX_CACHE_BLOCKS" ] && SERVER_CMD="$SERVER_CMD --max-cache-blocks $MAX_CACHE_BLOCKS"
[ -n "$EXTRA_ARGS" ] && SERVER_CMD="$SERVER_CMD $EXTRA_ARGS"

# Kill any existing server on our port
existing=$(lsof -ti:${PORT} 2>/dev/null || true)
if [ -n "$existing" ]; then
    echo ">>> Killing existing server on port ${PORT}" >&2
    kill $existing 2>/dev/null || true
    sleep 3
fi

# Start server
LOG="/tmp/vllm-mlx-toks.log"
echo ">>> Starting: $SERVER_CMD" >&2
eval "$SERVER_CMD" > "$LOG" 2>&1 &
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
        echo ">>> Server died" >&2
        tail -10 "$LOG" >&2
        echo "mean_toks:   0.0"
        echo "score:       0.0"
        exit 1
    fi
    elapsed=$(( $(date +%s) - start ))
    if [ "$elapsed" -gt "$timeout" ]; then
        echo ">>> TIMEOUT" >&2
        kill $VLLM_PID 2>/dev/null
        echo "mean_toks:   0.0"
        echo "score:       0.0"
        exit 1
    fi
    sleep 2
done

# Clear log so we only capture our test requests
> "$LOG"
sleep 1

# Diverse prompts to avoid trivial cache hits
PROMPTS=(
    "Explain the theory of general relativity in detail."
    "Write a short story about a robot learning to paint."
    "Describe the process of photosynthesis step by step."
    "What are the main differences between TCP and UDP?"
    "Explain how a compiler transforms source code into machine code."
    "Describe the history of the Roman Empire from founding to fall."
    "What is the mathematical basis of public key cryptography?"
    "Explain how neural networks learn through backpropagation."
    "Describe the water cycle and its impact on weather patterns."
    "What are the key principles of distributed systems design?"
    "Explain the double slit experiment and its implications."
    "Describe the architecture of a modern CPU pipeline."
    "What is the significance of the Turing test?"
    "Explain how CRISPR gene editing works."
    "Describe the main ideas in Adam Smith's Wealth of Nations."
    "What are the fundamental forces in physics?"
)

echo ">>> Sending $NUM_REQUESTS requests ($CONCURRENT concurrent, max $MAX_TOKENS tokens)..." >&2
eval_start=$(date +%s)

# Send requests concurrently
pids=()
for i in $(seq 0 $(( NUM_REQUESTS - 1 ))); do
    prompt="${PROMPTS[$((i % ${#PROMPTS[@]}))]}"
    (curl -s -X POST "http://localhost:${PORT}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -d "{\"model\": \"${MODEL_PATH}\", \"messages\": [{\"role\": \"user\", \"content\": \"${prompt}\"}], \"max_tokens\": ${MAX_TOKENS}, \"temperature\": 0.7}" \
        > /dev/null 2>&1) &
    pids+=($!)

    # Throttle to CONCURRENT
    if [ ${#pids[@]} -ge "$CONCURRENT" ]; then
        wait "${pids[0]}"
        pids=("${pids[@]:1}")
    fi
done
wait

eval_end=$(date +%s)
wall_time=$(( eval_end - eval_start ))

# Parse tok/s from server log
sleep 2
toks_lines=$(grep "tok/s)" "$LOG" | grep -oP '[\d.]+(?= tok/s)')

if [ -z "$toks_lines" ]; then
    echo ">>> No tok/s data found in server log" >&2
    tail -20 "$LOG" >&2
    kill $VLLM_PID 2>/dev/null || true
    echo "mean_toks:   0.0"
    echo "score:       0.0"
    exit 1
fi

# Calculate stats
stats=$(echo "$toks_lines" | awk '
    BEGIN { sum=0; min=99999; max=0; n=0; total_toks=0 }
    {
        sum += $1; n++;
        if ($1 < min) min = $1;
        if ($1 > max) max = $1;
    }
    END {
        mean = sum / n;
        printf "%.1f %.1f %.1f %d", mean, min, max, n
    }')

mean=$(echo "$stats" | awk '{print $1}')
min=$(echo "$stats" | awk '{print $2}')
max=$(echo "$stats" | awk '{print $3}')
count=$(echo "$stats" | awk '{print $4}')

echo ">>> $count requests completed in ${wall_time}s" >&2

# Kill server
kill $VLLM_PID 2>/dev/null || true

echo "mean_toks:   $mean"
echo "min_toks:    $min"
echo "max_toks:    $max"
echo "requests:    $count"
echo "wall_time_s: $wall_time"
echo "score:       $mean"

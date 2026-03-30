#!/bin/bash
# Run BFCL evaluations across MLX models via vllm-mlx.
# Starts vllm-mlx per model, runs stratified smoke test, then full eval.
#
# Usage: ./eval-mlx.sh [--full]
#   --full: run complete BFCL (default: stratified smoke test only)

set -euo pipefail

EVAL_DIR="$(cd "$(dirname "$0")" && pwd)"
RESULTS_DIR="$EVAL_DIR/bfcl/results"
DATE=$(date +%Y-%m-%d)
PORT=8091
FULL=false

[ "${1:-}" = "--full" ] && FULL=true

mkdir -p "$RESULTS_DIR"

models=(
    "qwen3.5-27b-4bit|/Users/benaskins/models/mlx/Qwen3.5-27B-4bit"
    "qwen3.5-27b-8bit|/Users/benaskins/models/mlx/Qwen3.5-27B-8bit"
    "qwen3.5-122b-5bit|/Users/benaskins/models/mlx/Qwen3.5-122B-A10B-5bit"
    "qwen3-30b-8bit|/Users/benaskins/models/mlx/Qwen3-30B-A3B-8bit"
)

start_server() {
    local model_path="$1"
    echo ">>> Starting vllm-mlx with $model_path"
    vllm-mlx serve "$model_path" \
        --port "$PORT" \
        --enable-auto-tool-choice \
        --tool-call-parser qwen3_coder \
        --reasoning-parser qwen3 \
        > /tmp/vllm-mlx.log 2>&1 &
    VLLM_PID=$!

    # Wait for server
    local timeout=120
    local start=$(date +%s)
    while true; do
        if curl -s "http://localhost:${PORT}/v1/models" > /dev/null 2>&1; then
            local elapsed=$(( $(date +%s) - start ))
            echo ">>> Server ready after ${elapsed}s"
            return 0
        fi
        if ! kill -0 $VLLM_PID 2>/dev/null; then
            echo ">>> Server died during startup"
            tail -10 /tmp/vllm-mlx.log
            return 1
        fi
        local elapsed=$(( $(date +%s) - start ))
        if [ "$elapsed" -gt "$timeout" ]; then
            echo ">>> Server startup timeout"
            kill $VLLM_PID 2>/dev/null
            return 1
        fi
        sleep 2
    done
}

stop_server() {
    if [ -n "${VLLM_PID:-}" ]; then
        kill $VLLM_PID 2>/dev/null || true
        wait $VLLM_PID 2>/dev/null || true
        sleep 2
    fi
}

run_eval() {
    local name="$1"
    local model_path="$2"
    local limit="$3"
    local outfile="$4"

    echo ">>> Running BFCL (limit $limit) for $name"
    local start=$(date +%s)
    go run ./bfcl/cmd/bfcl-run/ \
        -dir bfcl/ \
        -provider local \
        -url "http://localhost:${PORT}" \
        -model "$model_path" \
        -limit "$limit" \
        -workers 1 \
        -v \
        > "$outfile" 2>&1
    local elapsed=$(( $(date +%s) - start ))
    echo ">>> $name complete in ${elapsed}s"
}

echo "=== MLX Model BFCL Eval ==="
echo "Date: $DATE"
echo "Mode: $([ "$FULL" = true ] && echo "full" || echo "smoke test (3 per category)")"
echo "Models: ${#models[@]}"
echo ""

declare -a summary

for entry in "${models[@]}"; do
    IFS='|' read -r name path <<< "$entry"
    echo "=========================================="
    echo "Model: $name ($(date))"
    echo "=========================================="

    stop_server
    if ! start_server "$path"; then
        summary+=("${name}|FAILED|N/A")
        echo ">>> SKIPPING $name"
        continue
    fi

    # Smoke test first
    smoke_file="${RESULTS_DIR}/${name}-smoke-${DATE}.txt"
    run_eval "$name" "$path" 3 "$smoke_file"

    # Print smoke results
    echo "--- Smoke test ---"
    grep -E "^(simple|multiple|parallel_multiple|parallel|irrelevance|Accuracy):" "$smoke_file" || true
    accuracy=$(grep "^Accuracy:" "$smoke_file" | awk '{print $2}')
    echo ""

    if [ "$FULL" = true ]; then
        full_file="${RESULTS_DIR}/${name}-mlx-${DATE}.txt"
        run_eval "$name" "$path" 0 "$full_file"
        accuracy=$(grep "^Accuracy:" "$full_file" | awk '{print $2}')
        echo "--- Full results ---"
        grep -E "^(simple|multiple|parallel_multiple|parallel|irrelevance):" "$full_file" || true
        echo ""
    fi

    summary+=("${name}|${accuracy:-N/A}")
    stop_server
done

echo "=========================================="
echo "=== Summary ($(date)) ==="
echo "=========================================="
printf "%-25s %s\n" "MODEL" "ACCURACY"
printf "%-25s %s\n" "-----" "--------"
for entry in "${summary[@]}"; do
    IFS='|' read -r name acc <<< "$entry"
    printf "%-25s %s\n" "$name" "$acc"
done

echo ""
echo "Results in: ${RESULTS_DIR}/"
echo "End: $(date)"

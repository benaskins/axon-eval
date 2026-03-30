#!/bin/bash
# Overnight BFCL evaluation across local models.
# Swaps model in infra-models, waits for health, runs full BFCL suite.
#
# Usage: ./eval-overnight.sh
#
# Results are saved to bfcl/results/<model>-<date>.txt

set -euo pipefail

SPEC="/Users/benaskins/dev/hestia-core-infrastructure/aurelia/services/infra-models.yaml"
RESULTS_DIR="bfcl/results"
DATE=$(date +%Y-%m-%d)
BFCL_DIR="bfcl/"
WORKERS=4
URL="http://localhost:8090"

mkdir -p "$RESULTS_DIR"

# Order: smallest to largest for early results
models=(
    "qwen3-30b|/Users/benaskins/models/qwen3-30b.gguf|--parallel 4"
    "qwen3-coder-next|/Users/benaskins/models/qwen3-coder-next.gguf|--parallel 4"
    "minimax-m2.5-q4km|/Users/benaskins/.models/MiniMax-M2.5-Q4_K_M-00001-of-00004.gguf|--parallel 4"
    "qwen3.5-397b-mxfp4|/Users/benaskins/models/Qwen3.5-397B-A17B-MXFP4_MOE-00001-of-00006.gguf|--parallel 4"
)

swap_model() {
    local model_path="$1"
    local extra_args="$2"
    echo ">>> Swapping model to: $model_path"

    cat > "$SPEC" <<EOF
service:
  name: infra-models
  category: infra
  type: native
  command: /opt/homebrew/bin/llama-server -m ${model_path} --jinja --flash-attn on -ngl 999 --mlock ${extra_args}

network:
  port: 8090

routing:
  hostname: models.hestia.internal
  tls: true

health:
  type: http
  path: /health
  interval: 15s
  timeout: 5s
  grace_period: 600s
  unhealthy_threshold: 3

restart:
  policy: on-failure
  max_attempts: 3
  delay: 15s
  backoff: exponential
  max_delay: 5m
  reset_after: 10m

env:
  LLAMA_ARG_PORT: "8090"
EOF

    (cd /Users/benaskins/dev/hestia-core-infrastructure && just aurelia-sync 2>&1)
}

wait_for_healthy() {
    local timeout=600
    local start=$(date +%s)
    echo ">>> Waiting for model to load (timeout ${timeout}s)..."
    while true; do
        local health
        health=$(curl -s -o /dev/null -w "%{http_code}" "$URL/health" 2>/dev/null || echo "000")
        local elapsed=$(( $(date +%s) - start ))
        if [ "$health" = "200" ]; then
            echo ">>> Healthy after ${elapsed}s"
            echo "$elapsed"
            return 0
        fi
        if [ "$elapsed" -gt "$timeout" ]; then
            echo ">>> TIMEOUT after ${elapsed}s"
            return 1
        fi
        sleep 10
    done
}

run_eval() {
    local name="$1"
    local outfile="${RESULTS_DIR}/${name}-${DATE}.txt"
    echo ">>> Running BFCL for ${name}, output: ${outfile}"
    local start=$(date +%s)
    go run ./bfcl/cmd/bfcl-run/ \
        -dir "$BFCL_DIR" \
        -provider local \
        -url "$URL" \
        -workers "$WORKERS" \
        -v \
        > "$outfile" 2>&1
    local elapsed=$(( $(date +%s) - start ))
    echo ">>> ${name} eval complete in ${elapsed}s"
    echo "$elapsed"

    grep -E "^(simple|multiple|parallel_multiple|parallel|irrelevance):" "$outfile" || true
    echo ""
}

echo "=== Overnight BFCL Eval ==="
echo "Date: $DATE"
echo "Start: $(date)"
echo "Models: ${#models[@]}"
echo ""

declare -a summary

for entry in "${models[@]}"; do
    IFS='|' read -r name path args <<< "$entry"
    echo "=========================================="
    echo "Model: $name ($(date))"
    echo "=========================================="

    swap_model "$path" "$args"

    load_time=0
    eval_time=0

    if load_output=$(wait_for_healthy); then
        load_time=$(echo "$load_output" | tail -1)
        if eval_output=$(run_eval "$name"); then
            eval_time=$(echo "$eval_output" | tail -1)
        fi
        accuracy=$(grep "^Accuracy:" "${RESULTS_DIR}/${name}-${DATE}.txt" 2>/dev/null | awk '{print $2}' || echo "N/A")
        summary+=("${name}|${load_time}s|${eval_time}s|${accuracy}")
    else
        summary+=("${name}|FAILED|FAILED|N/A")
        echo ">>> SKIPPING ${name} (failed to load)"
    fi

    echo ""
done

echo "=========================================="
echo "=== Summary ($(date)) ==="
echo "=========================================="
printf "%-25s %-12s %-12s %s\n" "MODEL" "LOAD TIME" "EVAL TIME" "ACCURACY"
printf "%-25s %-12s %-12s %s\n" "-----" "---------" "---------" "--------"
for entry in "${summary[@]}"; do
    IFS='|' read -r name load eval acc <<< "$entry"
    printf "%-25s %-12s %-12s %s\n" "$name" "$load" "$eval" "$acc"
done

echo ""
echo "Results in: ${RESULTS_DIR}/"
ls -la "${RESULTS_DIR}/"*"${DATE}"* 2>/dev/null || echo "(no results)"
echo "End: $(date)"

echo ""
echo "=== Launching autotune ==="
echo "Starting Claude Code autotune session..."
cd /Users/benaskins/dev/lamina/axon-eval
claude -p "Read autotune/program.md and follow the instructions. Begin the autotune experiment loop. Today's date is $(date +%Y-%m-%d)." --allowedTools "Bash,Read,Write,Edit,Grep,Glob" > autotune/session-${DATE}.log 2>&1 &
echo "Autotune PID: $!"

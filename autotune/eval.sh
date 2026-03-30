#!/bin/bash
# Fixed evaluation harness. DO NOT MODIFY.
# Sources config.sh, applies server config, runs BFCL eval, outputs metrics.
#
# Output format (grep-friendly):
#   accuracy:    0.950
#   throughput:  0.290
#   avg_lat_ms:  6500
#   wall_time_s: 140
#   score:       0.827
#
# The "score" is the single metric to optimize (higher is better).
# score = accuracy * 0.7 + throughput_normalized * 0.3
# where throughput_normalized = min(1.0, cases_per_second / TARGET_THROUGHPUT)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EVAL_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
SPEC="/Users/benaskins/dev/hestia-core-infrastructure/aurelia/services/infra-models.yaml"

# Defaults (config.sh overrides these)
MODEL_PATH="/Users/benaskins/models/Qwen3.5-397B-A17B-MXFP4_MOE-00001-of-00006.gguf"
PARALLEL=4
CTX_SIZE=0
BATCH_SIZE=0
UBATCH_SIZE=0
THREADS=0
CACHE_TYPE_K=""
CACHE_TYPE_V=""
EXTRA_SERVER_ARGS=""
EVAL_URL="http://localhost:8090"
EVAL_MODEL="qwen3.5-397b"
EVAL_CATEGORY=""
EVAL_LIMIT=3
EVAL_WORKERS=1
TARGET_THROUGHPUT=0.15

# Source the tunable config
source "$SCRIPT_DIR/config.sh"

# Build server command
SERVER_CMD="/opt/homebrew/bin/llama-server -m ${MODEL_PATH} --jinja --flash-attn on -ngl 999 --mlock --parallel ${PARALLEL}"
[ "$CTX_SIZE" -gt 0 ] 2>/dev/null && SERVER_CMD="$SERVER_CMD --ctx-size $CTX_SIZE"
[ "$BATCH_SIZE" -gt 0 ] 2>/dev/null && SERVER_CMD="$SERVER_CMD --batch-size $BATCH_SIZE"
[ "$UBATCH_SIZE" -gt 0 ] 2>/dev/null && SERVER_CMD="$SERVER_CMD --ubatch-size $UBATCH_SIZE"
[ "$THREADS" -gt 0 ] 2>/dev/null && SERVER_CMD="$SERVER_CMD --threads $THREADS"
[ -n "$CACHE_TYPE_K" ] && SERVER_CMD="$SERVER_CMD --cache-type-k $CACHE_TYPE_K"
[ -n "$CACHE_TYPE_V" ] && SERVER_CMD="$SERVER_CMD --cache-type-v $CACHE_TYPE_V"
[ -n "$EXTRA_SERVER_ARGS" ] && SERVER_CMD="$SERVER_CMD $EXTRA_SERVER_ARGS"

# Check if server config changed
CURRENT_CMD=$(grep "command:" "$SPEC" 2>/dev/null | sed 's/.*command: //' || echo "")
if [ "$SERVER_CMD" != "$CURRENT_CMD" ]; then
    echo ">>> Server config changed, restarting..." >&2

    cat > "$SPEC" <<EOF
service:
  name: infra-models
  category: infra
  type: native
  command: ${SERVER_CMD}

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

    (cd /Users/benaskins/dev/hestia-core-infrastructure && just aurelia-sync 2>&1) >&2

    # Wait for healthy
    echo ">>> Waiting for model to load..." >&2
    timeout=600
    start=$(date +%s)
    while true; do
        health=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8090/health" 2>/dev/null || echo "000")
        elapsed=$(( $(date +%s) - start ))
        if [ "$health" = "200" ]; then
            echo ">>> Healthy after ${elapsed}s" >&2
            break
        fi
        if [ "$elapsed" -gt "$timeout" ]; then
            echo ">>> TIMEOUT waiting for server" >&2
            echo "accuracy:    0.000"
            echo "throughput:  0.000"
            echo "avg_lat_ms:  0"
            echo "wall_time_s: 0"
            echo "score:       0.000"
            exit 1
        fi
        sleep 5
    done
else
    echo ">>> Server config unchanged, skipping restart" >&2
fi

# Run eval
CATEGORY_FLAG=""
[ -n "$EVAL_CATEGORY" ] && CATEGORY_FLAG="-category $EVAL_CATEGORY"
echo ">>> Running BFCL (limit ${EVAL_LIMIT}, workers ${EVAL_WORKERS})..." >&2
eval_start=$(date +%s)

cd "$EVAL_DIR"
output=$(go run ./bfcl/cmd/bfcl-run/ \
    -dir bfcl/ \
    -provider local \
    -url "$EVAL_URL" \
    -model "$EVAL_MODEL" \
    -limit "$EVAL_LIMIT" \
    -workers "$EVAL_WORKERS" \
    $CATEGORY_FLAG \
    2>&1)

eval_end=$(date +%s)
wall_time=$(( eval_end - eval_start ))

# Parse results from overall summary
# Format: "Accuracy: 93.5%"  and  "Avg lat:  13116ms"  and  "Total:    15"  and  "Passed:   14"
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

#!/bin/bash
# eval-queue: sequential job runner with GPU-exclusive access.
# Watches ~/.eval-queue/pending/ for YAML job files, runs them one at a time.
# Manages vllm-mlx lifecycle, reuses server when consecutive jobs share a model.
#
# Job file format (YAML):
#   name: bfcl-122b-full
#   model: /Users/benaskins/models/mlx/Qwen3.5-122B-A10B-5bit
#   server: vllm-mlx          # or llama-server
#   server_args: --enable-auto-tool-choice --tool-call-parser qwen3_coder --reasoning-parser qwen3
#   command: go run ./bfcl/cmd/bfcl-run/ -dir bfcl/ -provider local -url http://localhost:${PORT} -model ${MODEL} -workers 1 -v
#   workdir: /Users/benaskins/dev/lamina/axon-eval
#   priority: 10              # lower = higher priority (default: 50)
#
# Usage:
#   eval-queue                 # run forever, processing jobs
#   eval-queue --once          # process pending jobs then exit
#   eval-queue --submit FILE   # copy a job file into pending/

set -euo pipefail

QUEUE_DIR="$HOME/.eval-queue"
PORT=8091
CURRENT_MODEL=""
SERVER_PID=""
ONCE=false

# Parse args
case "${1:-}" in
    --once) ONCE=true ;;
    --submit)
        if [ -z "${2:-}" ]; then
            echo "usage: eval-queue --submit FILE" >&2
            exit 1
        fi
        ts=$(date +%s)
        name=$(grep "^name:" "$2" | awk '{print $2}')
        prio=$(grep "^priority:" "$2" | awk '{print $2}')
        prio=${prio:-50}
        dest="${QUEUE_DIR}/pending/${prio}-${ts}-${name:-job}.yaml"
        cp "$2" "$dest"
        echo "Submitted: $dest"
        exit 0
        ;;
esac

# Simple YAML parser (reads key: value lines)
parse_yaml() {
    local file="$1" key="$2"
    grep "^${key}:" "$file" 2>/dev/null | sed "s/^${key}:[[:space:]]*//"
}

stop_server() {
    if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        echo "[queue] Stopping server (PID $SERVER_PID)"
        kill "$SERVER_PID" 2>/dev/null || true
        wait "$SERVER_PID" 2>/dev/null || true
        sleep 2
    fi
    SERVER_PID=""
    CURRENT_MODEL=""
}

start_server() {
    local model="$1" server_type="$2" server_args="$3"

    # Reuse if same model already loaded
    if [ "$model" = "$CURRENT_MODEL" ] && [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
        echo "[queue] Reusing server for $model"
        return 0
    fi

    stop_server

    echo "[queue] Starting $server_type for $(basename "$model")"

    case "$server_type" in
        vllm-mlx)
            vllm-mlx serve "$model" --port "$PORT" $server_args > "${QUEUE_DIR}/server.log" 2>&1 &
            SERVER_PID=$!
            ;;
        llama-server)
            /opt/homebrew/bin/llama-server -m "$model" --port "$PORT" $server_args > "${QUEUE_DIR}/server.log" 2>&1 &
            SERVER_PID=$!
            ;;
        *)
            echo "[queue] Unknown server type: $server_type" >&2
            return 1
            ;;
    esac

    # Wait for healthy
    local timeout=600
    local start=$(date +%s)
    while true; do
        if curl -s "http://localhost:${PORT}/v1/models" > /dev/null 2>&1 || \
           curl -s "http://localhost:${PORT}/health" > /dev/null 2>&1; then
            local elapsed=$(( $(date +%s) - start ))
            echo "[queue] Server ready after ${elapsed}s"
            CURRENT_MODEL="$model"
            return 0
        fi
        if ! kill -0 "$SERVER_PID" 2>/dev/null; then
            echo "[queue] Server died during startup"
            tail -5 "${QUEUE_DIR}/server.log" >&2
            SERVER_PID=""
            return 1
        fi
        local elapsed=$(( $(date +%s) - start ))
        if [ "$elapsed" -gt "$timeout" ]; then
            echo "[queue] Server startup timeout"
            stop_server
            return 1
        fi
        sleep 3
    done
}

run_job() {
    local job_file="$1"
    local job_name=$(basename "$job_file" .yaml)

    local name=$(parse_yaml "$job_file" "name")
    local model=$(parse_yaml "$job_file" "model")
    local server_type=$(parse_yaml "$job_file" "server")
    local server_args=$(parse_yaml "$job_file" "server_args")
    local command=$(parse_yaml "$job_file" "command")
    local workdir=$(parse_yaml "$job_file" "workdir")

    name=${name:-$job_name}
    server_type=${server_type:-vllm-mlx}
    workdir=${workdir:-.}

    echo ""
    echo "=========================================="
    echo "[queue] Job: $name ($(date))"
    echo "[queue] Model: $(basename "$model")"
    echo "=========================================="

    # Move to running
    mv "$job_file" "${QUEUE_DIR}/running/"
    local running_file="${QUEUE_DIR}/running/$(basename "$job_file")"

    # Start/reuse server
    if ! start_server "$model" "$server_type" "$server_args"; then
        echo "[queue] FAILED: server startup"
        mv "$running_file" "${QUEUE_DIR}/failed/"
        return 1
    fi

    # Substitute variables in command
    local resolved_cmd="${command//\$\{PORT\}/$PORT}"
    resolved_cmd="${resolved_cmd//\$\{MODEL\}/$model}"

    # Run
    local start=$(date +%s)
    local output_file="${QUEUE_DIR}/done/${name}-$(date +%Y%m%d-%H%M%S).log"

    echo "[queue] Running: $resolved_cmd"
    echo "[queue] Output: $output_file"

    if (cd "$workdir" && eval "$resolved_cmd") > "$output_file" 2>&1; then
        local elapsed=$(( $(date +%s) - start ))
        echo "[queue] DONE: $name (${elapsed}s)"
        mv "$running_file" "${QUEUE_DIR}/done/"
    else
        local elapsed=$(( $(date +%s) - start ))
        echo "[queue] FAILED: $name (${elapsed}s)"
        mv "$running_file" "${QUEUE_DIR}/failed/"
    fi
}

process_pending() {
    # Sort pending jobs by filename (priority prefix sorts naturally)
    local jobs=()
    for f in "${QUEUE_DIR}/pending/"*.yaml; do
        [ -f "$f" ] && jobs+=("$f")
    done

    if [ ${#jobs[@]} -eq 0 ]; then
        return 1 # nothing to do
    fi

    # Sort by filename (priority-timestamp-name.yaml)
    IFS=$'\n' sorted=($(sort <<<"${jobs[*]}")); unset IFS

    # Group by model to minimize reloads
    local by_model=()
    local other=()
    for f in "${sorted[@]}"; do
        local m=$(parse_yaml "$f" "model")
        if [ "$m" = "$CURRENT_MODEL" ]; then
            by_model+=("$f")
        else
            other+=("$f")
        fi
    done

    # Run same-model jobs first, then others
    for f in "${by_model[@]}" "${other[@]}"; do
        run_job "$f"
    done
}

# Cleanup on exit
trap 'echo "[queue] Shutting down..."; stop_server; exit 0' INT TERM

echo "[queue] eval-queue started"
echo "[queue] Watching: ${QUEUE_DIR}/pending/"
echo "[queue] Port: $PORT"
echo ""

if [ "$ONCE" = true ]; then
    process_pending || echo "[queue] No pending jobs"
    stop_server
    exit 0
fi

# Run forever
while true; do
    if ! process_pending; then
        sleep 10
    fi
done

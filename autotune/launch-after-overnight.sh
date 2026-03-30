#!/bin/bash
# Waits for the overnight eval to finish, then launches autotune.
set -euo pipefail

EVAL_DIR="/Users/benaskins/dev/lamina/axon-eval"
DATE=$(date +%Y-%m-%d)
LOG="$EVAL_DIR/bfcl/results/overnight-${DATE}.log"

echo "Waiting for overnight eval to complete..."
echo "Watching: $LOG"

# Wait for the overnight script to finish (look for "All evaluations complete")
while true; do
    if grep -q "All evaluations complete" "$LOG" 2>/dev/null; then
        echo "Overnight eval finished at $(date)"
        break
    fi
    sleep 30
done

echo "Launching autotune session..."
cd "$EVAL_DIR"
claude -p "Read autotune/program.md and follow the instructions exactly. Begin the autotune experiment loop. Today's date is ${DATE}." \
    --allowedTools "Bash,Read,Write,Edit,Grep,Glob" \
    > "autotune/session-${DATE}.log" 2>&1 &

echo "Autotune launched, PID: $!"
echo "Session log: autotune/session-${DATE}.log"

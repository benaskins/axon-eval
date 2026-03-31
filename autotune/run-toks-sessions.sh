#!/bin/bash
# Run tok/s autotune as short-lived Claude sessions.
# Each session reads prior results, runs N experiments, exits.

set -euo pipefail

ROUNDS=${1:-3}
EXPERIMENTS=${2:-10}
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DATE=$(date +%Y-%m-%d)

echo "=== Autotune tok/s: ${ROUNDS} rounds x ${EXPERIMENTS} experiments ==="
echo "Date: $DATE"
echo ""

for round in $(seq 1 "$ROUNDS"); do
    echo "=========================================="
    echo "Round ${round}/${ROUNDS} ($(date))"
    echo "=========================================="

    prior_count=0
    if [ -f "$SCRIPT_DIR/toks-results.tsv" ]; then
        prior_count=$(tail -n +2 "$SCRIPT_DIR/toks-results.tsv" | wc -l | tr -d ' ')
    fi
    echo "Prior experiments: $prior_count"

    timeout 20m claude -p "Read autotune/toks-program.md and follow the instructions.
Read autotune/toks-results.tsv to see what has already been tried (${prior_count} prior experiments).
Today's date is ${DATE}. This is round ${round} of ${ROUNDS}.

Run exactly ${EXPERIMENTS} experiments, then stop. Do NOT loop forever.
After ${EXPERIMENTS} experiments, print 'ROUND COMPLETE' and stop." \
        --allowedTools "Bash,Read,Write,Edit,Grep,Glob" \
        >> "$SCRIPT_DIR/toks-session-${DATE}.log" 2>&1 || true

    new_count=0
    if [ -f "$SCRIPT_DIR/toks-results.tsv" ]; then
        new_count=$(tail -n +2 "$SCRIPT_DIR/toks-results.tsv" | wc -l | tr -d ' ')
    fi
    added=$(( new_count - prior_count ))
    echo "Experiments this round: $added (total: $new_count)"
    echo ""
done

echo "=== Autotune tok/s complete ==="
echo "Results: $SCRIPT_DIR/toks-results.tsv"

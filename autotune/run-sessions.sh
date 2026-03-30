#!/bin/bash
# Run autotune as a series of short-lived Claude sessions.
# Each session reads prior results from mlx-results.tsv, runs N experiments, then exits.
# A new session starts fresh with full context.
#
# Usage: run-sessions.sh [rounds] [experiments_per_round]
#   rounds: number of sessions to run (default: 5)
#   experiments_per_round: experiments per session (default: 10)

set -euo pipefail

ROUNDS=${1:-5}
EXPERIMENTS=${2:-10}
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
DATE=$(date +%Y-%m-%d)

echo "=== Autotune: ${ROUNDS} rounds x ${EXPERIMENTS} experiments ==="
echo "Date: $DATE"
echo ""

for round in $(seq 1 "$ROUNDS"); do
    echo "=========================================="
    echo "Round ${round}/${ROUNDS} ($(date))"
    echo "=========================================="

    prior_count=0
    if [ -f "$SCRIPT_DIR/mlx-results.tsv" ]; then
        prior_count=$(tail -n +2 "$SCRIPT_DIR/mlx-results.tsv" | wc -l | tr -d ' ')
    fi
    echo "Prior experiments: $prior_count"

    timeout 20m claude -p "Read autotune/mlx-program.md and follow the instructions.
Read autotune/mlx-results.tsv to see what has already been tried (${prior_count} prior experiments).
Today's date is ${DATE}. This is round ${round} of ${ROUNDS}.

IMPORTANT changes from the program:
- Set TARGET_THROUGHPUT=0.50 (not 0.15) to leave room for throughput optimization.
- Run exactly ${EXPERIMENTS} experiments, then stop. Do NOT loop forever.
- Explore MAX_NUM_SEQS and EVAL_WORKERS together (1, 2, 4) for throughput.
- After ${EXPERIMENTS} experiments, print 'ROUND COMPLETE' and stop." \
        --allowedTools "Bash,Read,Write,Edit,Grep,Glob" \
        >> "$SCRIPT_DIR/mlx-session-${DATE}.log" 2>&1 || true

    new_count=0
    if [ -f "$SCRIPT_DIR/mlx-results.tsv" ]; then
        new_count=$(tail -n +2 "$SCRIPT_DIR/mlx-results.tsv" | wc -l | tr -d ' ')
    fi
    added=$(( new_count - prior_count ))
    echo "Experiments this round: $added (total: $new_count)"
    echo ""
done

echo "=== Autotune complete: ${ROUNDS} rounds ==="
echo "Results: $SCRIPT_DIR/mlx-results.tsv"
echo "Session log: $SCRIPT_DIR/mlx-session-${DATE}.log"

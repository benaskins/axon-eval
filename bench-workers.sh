#!/bin/bash
# Benchmark different worker counts against a fixed set of BFCL simple cases.
# Usage: ./bench-workers.sh

set -euo pipefail

LIMIT=20
URL="http://localhost:8090"
DIR="bfcl/"
CATEGORY="simple"

for workers in 1 2 4 8; do
    echo "=== Workers: $workers ==="
    start=$(date +%s)
    go run ./bfcl/cmd/bfcl-run/ \
        -dir "$DIR" \
        -provider local \
        -url "$URL" \
        -category "$CATEGORY" \
        -limit "$LIMIT" \
        -workers "$workers" 2>&1
    elapsed=$(( $(date +%s) - start ))
    echo "Wall time: ${elapsed}s"
    echo ""
done

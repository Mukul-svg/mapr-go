#!/usr/bin/env bash

# test_wc.sh — End-to-end word-count test for the MapReduce implementation.
# Requires: Linux or WSL (uses Unix domain sockets).
#
# Usage: bash scripts/test_wc.sh [<nworkers>]
#   nworkers: number of parallel workers to launch (default: 3)

set -o errexit
set -o nounset

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

N_WORKERS="${1:-3}"
SOCK="/tmp/mr-sock-$$"
OUT_DIR="$ROOT"

echo "=== Building binaries ==="
cd "$ROOT"
go build -o coordinator ./cmd/coordinator
go build -o worker ./cmd/worker
echo "  coordinator and worker built."

echo ""
echo "=== Cleaning up any old intermediate files ==="
rm -f mr-[0-9]*-[0-9]* mr-out-* mr-out-tmp-* mr-tmp-*

echo ""
echo "=== Starting coordinator (nReduce=10) ==="
./coordinator "$SOCK" testdata/pg-*.txt &
COORD_PID=$!
echo "  coordinator PID: $COORD_PID"

# Give the coordinator a moment to bind the socket.
sleep 1

echo ""
echo "=== Starting $N_WORKERS workers ==="
for i in $(seq 1 "$N_WORKERS"); do
    ./worker --app wc "$SOCK" &
    echo "  worker $i PID: $!"
done

echo ""
echo "=== Waiting for coordinator to finish ==="
wait "$COORD_PID"

echo ""
echo "=== Merging output files ==="
OUTPUT="mr-wc-final.txt"
sort mr-out-* > "$OUTPUT"
echo "  merged output written to $OUTPUT"

echo ""
echo "=== Sample output (first 20 lines) ==="
head -20 "$OUTPUT"

echo ""
LINE_COUNT=$(wc -l < "$OUTPUT")
echo "=== Total unique words: $LINE_COUNT ==="

if [ "$LINE_COUNT" -gt 0 ]; then
    echo ""
    echo "SUCCESS: Word count MapReduce completed!"
else
    echo "FAIL: Output is empty."
    exit 1
fi

echo ""
echo "=== Cleaning up ==="
rm -f coordinator worker mr-[0-9]*-[0-9]* mr-out-[0-9]* mr-tmp-* mr-out-tmp-*

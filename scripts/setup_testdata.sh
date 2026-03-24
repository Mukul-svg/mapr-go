#!/usr/bin/env bash

# setup_testdata.sh — Copy Project Gutenberg test files from the 6.5840 lab
# into the testdata/ directory.
#
# Usage: bash scripts/setup_testdata.sh <path-to-6.5840-src-main>
#
# Example:
#   bash scripts/setup_testdata.sh ~/6.5840/src/main

set -o errexit

if [ "$#" -lt 1 ]; then
    echo "Usage: bash scripts/setup_testdata.sh <path-to-6.5840-src-main>"
    echo ""
    echo "Example:"
    echo "  bash scripts/setup_testdata.sh ~/6.5840/src/main"
    exit 1
fi

SRC="$1"
DST="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/testdata"

if ls "$SRC"/pg-*.txt 1>/dev/null 2>&1; then
    cp "$SRC"/pg-*.txt "$DST/"
    echo "Copied $(ls "$DST"/pg-*.txt | wc -l) text files to testdata/"
else
    echo "Error: No pg-*.txt files found in $SRC"
    exit 1
fi

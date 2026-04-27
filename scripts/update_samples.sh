#!/usr/bin/env bash
# update_samples.sh — re-renders all samples/*.mmd files into samples/*.png
# using the native mmdg renderer.
#
# Usage: ./scripts/update_samples.sh
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SAMPLES_DIR="$REPO_ROOT/samples"

echo "Building mmdg..."
cd "$REPO_ROOT"
go build -o mmdg_bin ./cmd/mmdg

echo "Rendering samples from $SAMPLES_DIR ..."
echo

passed=0
failed=0

for mmd_file in "$SAMPLES_DIR"/*.mmd; do
  name="$(basename "${mmd_file%.mmd}")"
  png_file="$SAMPLES_DIR/${name}.png"

  printf "  %-35s ... " "$name"
  if ./mmdg_bin -i "$mmd_file" -o "$png_file" -e png -w 1600 -H 1200 -allowApproximate 2>/tmp/mmdg_${name}.log; then
    echo "ok"
    passed=$((passed + 1))
  else
    echo "FAILED (see /tmp/mmdg_${name}.log)"
    failed=$((failed + 1))
  fi
done

echo
echo "Done: $passed rendered, $failed failed"
if [[ $failed -gt 0 ]]; then
  exit 1
fi

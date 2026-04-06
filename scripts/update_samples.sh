#!/usr/bin/env bash
# update_samples.sh — re-renders all samples/*.mmd files into samples/*.png
# using the canonical mmdc (Mermaid JS CLI) renderer.
#
# Usage: ./scripts/update_samples.sh
#
# Requirements: mmdc must be installed (npm install -g @mermaid-js/mermaid-cli)
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SAMPLES_DIR="$REPO_ROOT/samples"

if ! command -v mmdc > /dev/null 2>&1; then
  echo "error: mmdc not found in PATH. Install with: npm install -g @mermaid-js/mermaid-cli" >&2
  exit 1
fi

echo "mmdc $(mmdc --version 2>&1 | head -1)"
echo "Rendering samples from $SAMPLES_DIR ..."
echo

passed=0
failed=0

for mmd_file in "$SAMPLES_DIR"/*.mmd; do
  name="$(basename "${mmd_file%.mmd}")"
  png_file="$SAMPLES_DIR/${name}.png"

  printf "  %-35s ... " "$name"
  if mmdc -i "$mmd_file" -o "$png_file" -e png -b white -q 2>/tmp/mmdc_${name}.log; then
    echo "ok"
    ((passed++))
  else
    echo "FAILED (see /tmp/mmdc_${name}.log)"
    ((failed++))
  fi
done

echo
echo "Done: $passed rendered, $failed failed"
if [[ $failed -gt 0 ]]; then
  exit 1
fi

#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat <<'EOF'
Usage: ./scripts/report_png_parity_gaps.sh [existing-go-test-log]

Without an argument, runs:
  MMDG_PNG_CONFORMANCE=1 go test -run TestPNGConformanceAgainstMMDC -count=1 -v ./...

Then parses the PNG conformance lines, sorts fixtures by mismatch, and groups
the remaining >0.10 parity gaps by diagram family.
EOF
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

LOG_FILE="${1:-}"
TEMP_LOG=""

if [[ -z "$LOG_FILE" ]]; then
  TEMP_LOG="$(mktemp)"
  LOG_FILE="$TEMP_LOG"
  echo "Running PNG conformance sweep..." >&2
  set +e
  (
    cd "$ROOT_DIR"
    MMDG_PNG_CONFORMANCE=1 go test -run TestPNGConformanceAgainstMMDC -count=1 -v ./...
  ) | tee "$LOG_FILE"
  TEST_STATUS=${PIPESTATUS[0]}
  set -e
  if [[ $TEST_STATUS -ne 0 ]]; then
    echo "go test exited with status $TEST_STATUS; continuing with parsed report." >&2
  fi
elif [[ ! -f "$LOG_FILE" ]]; then
  echo "log file not found: $LOG_FILE" >&2
  exit 1
fi

python3 - "$LOG_FILE" <<'PY'
import collections
import pathlib
import re
import sys

target = 0.10
pattern = re.compile(r"fixture=([a-z0-9_]+)\s+png_mismatch=([0-9.]+)")

entries = []
for line in pathlib.Path(sys.argv[1]).read_text().splitlines():
    match = pattern.search(line)
    if not match:
        continue
    fixture = match.group(1)
    mismatch = float(match.group(2))
    family = fixture.split("_", 1)[0]
    entries.append((fixture, family, mismatch))

if not entries:
    print("No PNG conformance entries found.", file=sys.stderr)
    sys.exit(1)

entries.sort(key=lambda item: item[2], reverse=True)
over_target = [entry for entry in entries if entry[2] > target]
families = collections.defaultdict(list)
for fixture, family, mismatch in over_target:
    families[family].append((fixture, mismatch))

print("PNG parity summary")
print(f"Fixtures measured: {len(entries)}")
print(f"Fixtures above target ({target:.2f}): {len(over_target)}")
print()

print("Worst fixtures:")
for fixture, _, mismatch in entries[:10]:
    marker = " !" if mismatch > target else ""
    print(f"  {fixture:<20} {mismatch:0.4f}{marker}")

if not over_target:
    print()
    print("All fixtures are at or below the 0.10 target.")
    sys.exit(0)

print()
print("Families above target:")
ranked_families = sorted(
    families.items(),
    key=lambda item: max(mismatch for _, mismatch in item[1]),
    reverse=True,
)
for family, family_entries in ranked_families:
    worst = max(mismatch for _, mismatch in family_entries)
    print(f"  {family:<12} worst={worst:0.4f} count={len(family_entries)}")

print()
print("Suggested fix order:")
for index, (family, family_entries) in enumerate(ranked_families, start=1):
    fixtures = ", ".join(
        f"{fixture}={mismatch:0.4f}"
        for fixture, mismatch in sorted(family_entries, key=lambda item: item[1], reverse=True)
    )
    print(f"  {index}. {family}: {fixtures}")
PY

if [[ -n "$TEMP_LOG" ]]; then
  rm -f "$TEMP_LOG"
fi

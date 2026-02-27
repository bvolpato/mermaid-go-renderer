#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${1:-/tmp}"
WIDTH="${WIDTH:-1600}"
HEIGHT="${HEIGHT:-1200}"
MMDR_BIN="${MMDR_BIN:-}"

mkdir -p "$OUT_DIR"

if [ -z "$MMDR_BIN" ]; then
  LOCAL_MMDR="$REPO_ROOT/../mermaid-rs-renderer/target/release/mmdr"
  if [ -x "$LOCAL_MMDR" ]; then
    MMDR_BIN="$LOCAL_MMDR"
  elif command -v mmdr >/dev/null 2>&1; then
    MMDR_BIN="$(command -v mmdr)"
  fi
fi
if [ -z "$MMDR_BIN" ]; then
  echo "error: mmdr not found (set MMDR_BIN or install mmdr)" >&2
  exit 1
fi
if ! command -v mmdc >/dev/null 2>&1; then
  echo "error: mmdc not found in PATH" >&2
  exit 1
fi

cd "$REPO_ROOT"
TMP_MMDG_BIN="$OUT_DIR/mmdg-triplet-bin"
go build -o "$TMP_MMDG_BIN" ./cmd/mmdg

mapfile -t FILES < <(rg --files "$REPO_ROOT/testdata" --glob '*.mmd' | sort)
if [ "${#FILES[@]}" -eq 0 ]; then
  echo "no .mmd files found under $REPO_ROOT/testdata" >&2
  exit 1
fi

for file in "${FILES[@]}"; do
  rel="${file#$REPO_ROOT/testdata/}"
  stem="${rel%.mmd}"
  safe="${stem//\//_}"
  go_out="$OUT_DIR/${safe}_go.svg"
  rust_out="$OUT_DIR/${safe}_rust.svg"
  cli_out="$OUT_DIR/${safe}_cli.svg"

  echo "rendering $rel"
  "$TMP_MMDG_BIN" -i "$file" -o "$go_out" --allowApproximate
  "$MMDR_BIN" -i "$file" -o "$rust_out" -e svg -w "$WIDTH" -H "$HEIGHT"
  mmdc -i "$file" -o "$cli_out" -e svg -w "$WIDTH" -H "$HEIGHT" -b white -q
done

echo "triplet render complete: $OUT_DIR"

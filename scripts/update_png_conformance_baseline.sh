#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$REPO_ROOT"

MMDG_PNG_CONFORMANCE=1 \
MMDG_PNG_UPDATE_BASELINE=1 \
go test -run TestPNGConformanceAgainstMMDC -count=1 -v

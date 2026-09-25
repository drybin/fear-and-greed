#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"; cd "$ROOT"
BIN="${BIN:-$ROOT/bin/cli}"; DATA_DIR="${DATA_DIR:-$ROOT/data/research-futures-v1}"
START="${START:-2025-05-03}"; END="${END:-2026-08-01}"; REVISION="$(git rev-parse --short=12 HEAD)"
OUTPUT="${OUTPUT:-$DATA_DIR/diagnostics/btc-lead-lag-rr3-v1-${START}_${END}-${REVISION}.json}"
[[ -x "$BIN" ]] || { echo "ERROR: build bin/cli first (make build-cli)." >&2; exit 1; }
"$BIN" research-validate btc-lead-lag-rr3 --candle-dir "$DATA_DIR" --start "$START" --end "$END" --output "$OUTPUT"
echo "BTC lead-lag RR3 backtest complete: $OUTPUT"

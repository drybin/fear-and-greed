#!/usr/bin/env bash
# Descriptive BTC-to-alt lead-lag measurement on existing USD-M hourly data.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
BIN="${BIN:-$ROOT/bin/cli}"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-futures-v1}"
SYMBOLS_FILE="${SYMBOLS_FILE:-$ROOT/scripts/symbols_futures_top50.txt}"
START="${START:-2025-05-03}"
END="${END:-2026-08-01}"
IMPULSE_PERCENT="${IMPULSE_PERCENT:-1}"
REVISION="$(git rev-parse --short=12 HEAD)"
OUTPUT="${OUTPUT:-$DATA_DIR/diagnostics/btc-lead-lag-v1-${START}_${END}-${REVISION}.json}"

if [[ ! -x "$BIN" ]]; then
  echo "ERROR: build bin/cli first (make build-cli)." >&2
  exit 1
fi

"$BIN" research-validate btc-lead-lag \
  --symbols "$SYMBOLS_FILE" \
  --candle-dir "$DATA_DIR" \
  --start "$START" \
  --end "$END" \
  --impulse-percent "$IMPULSE_PERCENT" \
  --output "$OUTPUT"

echo "BTC lead-lag diagnostic complete: $OUTPUT"

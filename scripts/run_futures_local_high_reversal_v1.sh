#!/usr/bin/env bash
# Independent USD-M futures research: causal 20-hour local-high short reversal.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
BIN="${BIN:-$ROOT/bin/cli}"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-futures-v1}"
SYMBOLS_FILE="${SYMBOLS_FILE:-$ROOT/scripts/symbols_futures_top50.txt}"
SINCE="${SINCE:-2024-07-01}"
CUTOFF="${CUTOFF:-2026-08-01}"
UNTIL="${UNTIL:-2026-07-31}"
GOMEMLIMIT="${GOMEMLIMIT:-512MiB}"
GOGC="${GOGC:-20}"

if [[ ! -x "$BIN" ]]; then echo "ERROR: build bin/cli first (make build-cli)." >&2; exit 1; fi
if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then echo "ERROR: commit tracked changes before the research run." >&2; exit 1; fi

REVISION="$(git rev-parse --short=12 HEAD)"
RUN_DIR="${RUN_DIR:-$DATA_DIR/runs/${CUTOFF}-${REVISION}}"
mkdir -p "$DATA_DIR" "$RUN_DIR"

while IFS= read -r symbol || [[ -n "$symbol" ]]; do
  symbol="${symbol//[[:space:]]/}"
  [[ -z "$symbol" || "$symbol" == \#* ]] && continue
  candle="$DATA_DIR/${symbol}_futures.csv"
  funding="$DATA_DIR/${symbol}_futures_funding.csv"
  [[ -s "$candle" ]] || "$BIN" fetch-data --market futures --symbol "$symbol" --interval 1h --since "$SINCE" --until "$UNTIL" --dir "$DATA_DIR" --no-progress
  [[ -s "$funding" ]] || "$BIN" fetch-funding --symbol "$symbol" --since "$SINCE" --until "$CUTOFF" --dir "$DATA_DIR"
done < "$SYMBOLS_FILE"

"$BIN" research-validate verify --workdir "$ROOT"
"$BIN" research-validate prepare --market futures --input-interval 1h --suite futures-local-high-reversal-v1 --symbols "$SYMBOLS_FILE" --candle-dir "$DATA_DIR" --manifest "$RUN_DIR/manifest.json" --cutoff "$CUTOFF" --seed 42 --workdir "$ROOT"
env GOMEMLIMIT="$GOMEMLIMIT" GOGC="$GOGC" "$BIN" research-validate development --manifest "$RUN_DIR/manifest.json" --candle-dir "$DATA_DIR" --output "$RUN_DIR/output" --workdir "$ROOT"
env GOMEMLIMIT="$GOMEMLIMIT" GOGC="$GOGC" "$BIN" research-validate freeze --manifest "$RUN_DIR/manifest.json" --candle-dir "$DATA_DIR" --output "$RUN_DIR/output" --workdir "$ROOT"
env GOMEMLIMIT="$GOMEMLIMIT" GOGC="$GOGC" "$BIN" research-validate review --existing-development --manifest "$RUN_DIR/manifest.json" --candle-dir "$DATA_DIR" --output "$RUN_DIR/output" --workdir "$ROOT"

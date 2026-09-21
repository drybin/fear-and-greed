#!/usr/bin/env bash
# Independent USD-M futures research: high-volume sell-off reversal long at 3R.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
BIN="${BIN:-$ROOT/bin/cli}"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-futures-v1}"
SYMBOLS_FILE="${SYMBOLS_FILE:-$ROOT/scripts/symbols_futures_top50.txt}"
CUTOFF="${CUTOFF:-2026-08-01}"
GOMEMLIMIT="${GOMEMLIMIT:-512MiB}"
GOGC="${GOGC:-20}"

if [[ ! -x "$BIN" ]]; then echo "ERROR: build bin/cli first (make build-cli)." >&2; exit 1; fi
if [[ -n "$(git status --porcelain --untracked-files=no)" ]]; then echo "ERROR: commit tracked changes before the research run." >&2; exit 1; fi

REVISION="$(git rev-parse --short=12 HEAD)"
RUN_DIR="${RUN_DIR:-$DATA_DIR/runs/volume-reversal-rr3-${CUTOFF}-${REVISION}}"
mkdir -p "$RUN_DIR"

"$BIN" research-validate verify --workdir "$ROOT"
"$BIN" research-validate prepare --market futures --input-interval 1h --suite futures-volume-reversal-rr3-v1 --symbols "$SYMBOLS_FILE" --candle-dir "$DATA_DIR" --manifest "$RUN_DIR/manifest.json" --cutoff "$CUTOFF" --seed 42 --workdir "$ROOT"
env GOMEMLIMIT="$GOMEMLIMIT" GOGC="$GOGC" "$BIN" research-validate development --manifest "$RUN_DIR/manifest.json" --candle-dir "$DATA_DIR" --output "$RUN_DIR/output" --workdir "$ROOT"
env GOMEMLIMIT="$GOMEMLIMIT" GOGC="$GOGC" "$BIN" research-validate freeze --manifest "$RUN_DIR/manifest.json" --candle-dir "$DATA_DIR" --output "$RUN_DIR/output" --workdir "$ROOT"
env GOMEMLIMIT="$GOMEMLIMIT" GOGC="$GOGC" "$BIN" research-validate review --existing-development --manifest "$RUN_DIR/manifest.json" --candle-dir "$DATA_DIR" --output "$RUN_DIR/output" --workdir "$ROOT"

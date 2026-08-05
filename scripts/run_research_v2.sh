#!/usr/bin/env bash
# Complete resumable protocol-v2 workflow.
#
# Default behavior: fetch -> verify -> prepare -> development -> freeze.
# The one-time holdout is only opened when AUTHORIZE_HOLDOUT=1 is explicit.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

CUTOFF="${CUTOFF:-2026-08-01}"
SINCE="${SINCE:-2024-07-01}"
UNTIL="${UNTIL:-2026-07-31}"
SEED="${SEED:-42}"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-v2}"
SYMBOLS_FILE="${SYMBOLS_FILE:-$ROOT/scripts/symbols_top50.txt}"
AUTHORIZE_HOLDOUT="${AUTHORIZE_HOLDOUT:-0}"
SKIP_FETCH="${SKIP_FETCH:-0}"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "ERROR: protocol-v2 requires a clean Git worktree." >&2
  echo "Commit the exact implementation and keep generated data under data/." >&2
  exit 1
fi

REVISION="$(git rev-parse --short=12 HEAD)"
RUN_DIR="${RUN_DIR:-$DATA_DIR/runs/${CUTOFF}-${REVISION}}"
BIN="${BIN:-$DATA_DIR/bin/fear-and-greed}"
MANIFEST="${MANIFEST:-$RUN_DIR/manifest.json}"
OUTPUT="${OUTPUT:-$RUN_DIR/output}"
LOG_FILE="$RUN_DIR/workflow.log"

mkdir -p "$(dirname "$BIN")" "$RUN_DIR"

run_logged() {
  printf '\n[%s] RUN:' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" | tee -a "$LOG_FILE"
  printf ' %q' "$@" | tee -a "$LOG_FILE"
  printf '\n' | tee -a "$LOG_FILE"
  "$@" 2>&1 | tee -a "$LOG_FILE"
}

echo "Protocol-v2 run: cutoff=$CUTOFF revision=$REVISION"
echo "Run directory: $RUN_DIR"

if [[ "$SKIP_FETCH" != "1" ]]; then
  BIN="$BIN" DATA_DIR="$DATA_DIR" SYMBOLS_FILE="$SYMBOLS_FILE" \
    SINCE="$SINCE" UNTIL="$UNTIL" INTERVAL=1m MARKET=spot BUILD=1 \
    "$ROOT/scripts/fetch_research_v2_top50.sh"
else
  run_logged go build -o "$BIN" ./cmd/cli/...
fi

run_logged "$BIN" research-validate verify --workdir "$ROOT"

run_logged "$BIN" research-validate prepare \
  --symbols "$SYMBOLS_FILE" \
  --candle-dir "$DATA_DIR" \
  --manifest "$MANIFEST" \
  --cutoff "$CUTOFF" \
  --seed "$SEED" \
  --workdir "$ROOT"

run_logged "$BIN" research-validate development \
  --manifest "$MANIFEST" \
  --candle-dir "$DATA_DIR" \
  --output "$OUTPUT" \
  --workdir "$ROOT"

run_logged "$BIN" research-validate freeze \
  --manifest "$MANIFEST" \
  --candle-dir "$DATA_DIR" \
  --output "$OUTPUT" \
  --workdir "$ROOT"

if [[ "$AUTHORIZE_HOLDOUT" != "1" ]]; then
  echo
  echo "Development and freeze completed. Holdout remains locked."
  echo "Review: $OUTPUT/protocol-v2"
  echo "To explicitly open holdout, rerun with AUTHORIZE_HOLDOUT=1 SKIP_FETCH=1."
  exit 0
fi

run_logged "$BIN" research-validate final \
  --authorize-holdout \
  --manifest "$MANIFEST" \
  --candle-dir "$DATA_DIR" \
  --output "$OUTPUT" \
  --workdir "$ROOT"

echo
echo "Protocol-v2 workflow completed, including the one-time holdout."
echo "Artifacts: $OUTPUT/protocol-v2"

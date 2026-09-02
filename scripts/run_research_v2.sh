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
SUITE="${SUITE:-core-v2}"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-v2}"
SYMBOLS_FILE="${SYMBOLS_FILE:-$ROOT/scripts/symbols_top50.txt}"
AUTHORIZE_HOLDOUT="${AUTHORIZE_HOLDOUT:-0}"
FINAL_ONLY="${FINAL_ONLY:-0}"
ORCHESTRATION_UPGRADE="${ORCHESTRATION_UPGRADE:-0}"
SKIP_FETCH="${SKIP_FETCH:-0}"
SKIP_VERIFY="${SKIP_VERIFY:-0}"
VERIFY_DOCKER_IMAGE="${VERIFY_DOCKER_IMAGE:-golang:1.22}"
# The in-process evaluator reads large 1m candle windows. Conservative GC
# defaults keep a VPS run below its memory limit; callers may override them.
GOMEMLIMIT="${GOMEMLIMIT:-512MiB}"
GOGC="${GOGC:-20}"

TRACKED_STATUS="$(git status --porcelain --untracked-files=no)"
UNTRACKED_SOURCE="$(git ls-files --others --exclude-standard -- '*.go' 'go.mod' 'go.sum')"
if [[ -n "$TRACKED_STATUS" || -n "$UNTRACKED_SOURCE" ]]; then
  echo "ERROR: protocol-v2 requires a clean Git worktree." >&2
  echo "Commit tracked changes and any untracked Go source before running." >&2
  exit 1
fi

REVISION="$(git rev-parse --short=12 HEAD)"
RUN_DIR="${RUN_DIR:-$DATA_DIR/runs/${CUTOFF}-${REVISION}}"
BIN="${BIN:-$ROOT/bin/cli}"
MANIFEST="${MANIFEST:-$RUN_DIR/manifest.json}"
OUTPUT="${OUTPUT:-$RUN_DIR/output}"
LOG_FILE="$RUN_DIR/workflow.log"

mkdir -p "$RUN_DIR"

if [[ ! -x "$BIN" ]]; then
  echo "ERROR: executable CLI not found: $BIN" >&2
  echo "Build it first or pass BIN=/absolute/path/to/fear-and-greed." >&2
  exit 1
fi

run_logged() {
  printf '\n[%s] RUN:' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" | tee -a "$LOG_FILE"
  printf ' %q' "$@" | tee -a "$LOG_FILE"
  printf '\n' | tee -a "$LOG_FILE"
  "$@" 2>&1 | tee -a "$LOG_FILE"
}

echo "Protocol-v2 run: cutoff=$CUTOFF revision=$REVISION"
echo "Research suite: $SUITE"
echo "Run directory: $RUN_DIR"

if [[ "$FINAL_ONLY" != "1" && "$SKIP_FETCH" != "1" ]]; then
  BIN="$BIN" DATA_DIR="$DATA_DIR" SYMBOLS_FILE="$SYMBOLS_FILE" \
    SINCE="$SINCE" UNTIL="$UNTIL" INTERVAL=1m MARKET=spot \
    "$ROOT/scripts/fetch_research_v2_top50.sh"
fi

if [[ "$FINAL_ONLY" == "1" ]]; then
  echo "Preparation and development skipped explicitly with FINAL_ONLY=1."
elif [[ "$SKIP_VERIFY" == "1" ]]; then
  echo "Verification skipped explicitly with SKIP_VERIFY=1."
elif command -v go >/dev/null 2>&1; then
  run_logged "$BIN" research-validate verify --workdir "$ROOT"
elif command -v docker >/dev/null 2>&1; then
  run_logged docker run --rm \
    -v "$ROOT:/app" \
    -w /app \
    "$VERIFY_DOCKER_IMAGE" \
    go test ./internal/research/... ./internal/strategy/...
else
  echo "ERROR: verification requires Go or Docker." >&2
  echo "If the exact commit was already tested during your manual build, rerun with SKIP_VERIFY=1." >&2
  exit 1
fi

if [[ "$FINAL_ONLY" != "1" ]]; then
  run_logged "$BIN" research-validate prepare \
    --symbols "$SYMBOLS_FILE" \
    --candle-dir "$DATA_DIR" \
    --manifest "$MANIFEST" \
    --cutoff "$CUTOFF" \
    --seed "$SEED" \
    --suite "$SUITE" \
    --workdir "$ROOT"

  run_logged env GOMEMLIMIT="$GOMEMLIMIT" GOGC="$GOGC" "$BIN" research-validate development \
    --manifest "$MANIFEST" \
    --candle-dir "$DATA_DIR" \
    --output "$OUTPUT" \
    --workdir "$ROOT"

  run_logged "$BIN" research-validate freeze \
    --manifest "$MANIFEST" \
    --candle-dir "$DATA_DIR" \
    --output "$OUTPUT" \
    --workdir "$ROOT"
fi

run_logged "$BIN" research-validate review \
  --existing-development \
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

FINAL_ARGS=()
if [[ "$ORCHESTRATION_UPGRADE" == "1" ]]; then
  FINAL_ARGS+=(--orchestration-upgrade)
fi

run_logged "$BIN" research-validate final \
  --authorize-holdout \
  "${FINAL_ARGS[@]}" \
  --manifest "$MANIFEST" \
  --candle-dir "$DATA_DIR" \
  --output "$OUTPUT" \
  --workdir "$ROOT"

echo
echo "Protocol-v2 workflow completed, including the one-time holdout."
echo "Artifacts: $OUTPUT/protocol-v2"

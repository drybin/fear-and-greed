#!/usr/bin/env bash
# Run one frozen relative-strength regime ablation without refetching candles.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

: "${RESEARCH_MANIFEST:?set RESEARCH_MANIFEST to the source protocol-v2 manifest}"

BIN="${BIN:-$ROOT/bin/cli}"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-v2}"
REGIME_MODES="${REGIME_MODES:-both btc-ema breadth none}"
ENTRY_MODE="${ENTRY_MODE:-weekly-open}"
START="${START:-}"
END="${END:-}"
VERIFY_DOCKER_IMAGE="${VERIFY_DOCKER_IMAGE:-golang:1.22}"
REVISION="$(git rev-parse --short=12 HEAD)"
RUN_ROOT="${RUN_ROOT:-$DATA_DIR/portfolio-runs/regime-ablation-$REVISION}"

TRACKED_STATUS="$(git status --porcelain --untracked-files=no)"
UNTRACKED_SOURCE="$(git ls-files --others --exclude-standard -- '*.go' 'go.mod' 'go.sum')"
if [[ -n "$TRACKED_STATUS" || -n "$UNTRACKED_SOURCE" ]]; then
  echo "ERROR: portfolio ablation requires a clean Git worktree." >&2
  exit 1
fi
if [[ ! -x "$BIN" ]]; then
  echo "ERROR: executable CLI not found: $BIN (run: make build-cli)" >&2
  exit 1
fi
if [[ ! -f "$RESEARCH_MANIFEST" ]]; then
  echo "ERROR: source manifest not found: $RESEARCH_MANIFEST" >&2
  exit 1
fi
case "$ENTRY_MODE" in
  weekly-open|trend-pullback) ;;
  *)
    echo "ERROR: unsupported entry mode: $ENTRY_MODE" >&2
    exit 1
    ;;
esac
if [[ -n "$START" || -n "$END" ]]; then
  if [[ -z "$START" || -z "$END" ]]; then
    echo "ERROR: set both START and END for a custom evaluation window." >&2
    exit 1
  fi
  PREPARE_RANGE_ARGS=(--start "$START" --end "$END")
else
  PREPARE_RANGE_ARGS=()
fi

mkdir -p "$RUN_ROOT"

run_logged() {
  local log_file="$1"
  shift
  printf '\n[%s] RUN:' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" | tee -a "$log_file"
  printf ' %q' "$@" | tee -a "$log_file"
  printf '\n' | tee -a "$log_file"
  "$@" 2>&1 | tee -a "$log_file"
}

LOG_FILE="$RUN_ROOT/workflow.log"
if [[ "${SKIP_VERIFY:-0}" != "1" ]]; then
  run_logged "$LOG_FILE" docker run --rm -v "$ROOT:/app" -w /app "$VERIFY_DOCKER_IMAGE" go test ./internal/research/... ./internal/strategy/...
fi

for mode in $REGIME_MODES; do
  case "$mode" in
    both|btc-ema|breadth|none) ;;
    *)
      echo "ERROR: unsupported regime mode: $mode" >&2
      exit 1
      ;;
  esac
  MODE_DIR="$RUN_ROOT/$mode"
  MANIFEST="$MODE_DIR/manifest.json"
  REPORT="$MODE_DIR/report.json"
  mkdir -p "$MODE_DIR"
  run_logged "$LOG_FILE" "$BIN" research-validate portfolio-prepare \
    --research-manifest "$RESEARCH_MANIFEST" \
    --manifest "$MANIFEST" \
    --regime-mode "$mode" \
    --entry-mode "$ENTRY_MODE" \
    "${PREPARE_RANGE_ARGS[@]}" \
    --workdir "$ROOT"
  run_logged "$LOG_FILE" env GOMEMLIMIT=512MiB GOGC=20 "$BIN" research-validate portfolio-run \
    --manifest "$MANIFEST" \
    --candle-dir "$DATA_DIR" \
    --output "$REPORT" \
    --workdir "$ROOT"
done

echo
echo "Regime ablation complete: $RUN_ROOT"

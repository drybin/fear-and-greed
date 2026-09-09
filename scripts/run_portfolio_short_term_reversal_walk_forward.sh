#!/usr/bin/env bash
# Evaluate two frozen short-term reversal candidates on five pre-holdout
# windows. The script reads existing candles only and never opens holdout.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

: "${RESEARCH_MANIFEST:?set RESEARCH_MANIFEST to the source protocol-v2 manifest}"

BIN="${BIN:-$ROOT/bin/cli}"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-v2}"
REVISION="$(git rev-parse --short=12 HEAD)"
RUN_ROOT="${RUN_ROOT:-$DATA_DIR/portfolio-runs/short-term-reversal-walk-forward-$REVISION}"
VERIFY_DOCKER_IMAGE="${VERIFY_DOCKER_IMAGE:-golang:1.22}"

TRACKED_STATUS="$(git status --porcelain --untracked-files=no)"
UNTRACKED_SOURCE="$(git ls-files --others --exclude-standard -- '*.go' 'go.mod' 'go.sum')"
if [[ -n "$TRACKED_STATUS" || -n "$UNTRACKED_SOURCE" ]]; then
  echo "ERROR: short-term reversal workflow requires a clean Git worktree." >&2
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

CANDIDATES=(5 10)
WINDOWS=(
  "01-2025-02-01_2025-05-03:2025-02-01:2025-05-03"
  "02-2025-05-03_2025-08-06:2025-05-03:2025-08-06"
  "03-2025-08-06_2025-11-04:2025-08-06:2025-11-04"
  "04-2025-11-04_2026-02-03:2025-11-04:2026-02-03"
  "05-2026-02-03_2026-05-03:2026-02-03:2026-05-03"
)

mkdir -p "$RUN_ROOT"
LOG_FILE="$RUN_ROOT/workflow.log"

run_logged() {
  printf '\n[%s] RUN:' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" | tee -a "$LOG_FILE"
  printf ' %q' "$@" | tee -a "$LOG_FILE"
  printf '\n' | tee -a "$LOG_FILE"
  "$@" 2>&1 | tee -a "$LOG_FILE"
}

run_logged docker run --rm -v "$ROOT:/app" -w /app "$VERIFY_DOCKER_IMAGE" go test ./internal/research/... ./internal/strategy/...

for top_k in "${CANDIDATES[@]}"; do
  candidate_dir="short-term-reversal-top${top_k}"
  for window in "${WINDOWS[@]}"; do
    IFS=: read -r label start end <<<"$window"
    output_dir="$RUN_ROOT/$candidate_dir/$label"
    manifest="$output_dir/manifest.json"
    report="$output_dir/report.json"
    mkdir -p "$output_dir"
    run_logged "$BIN" research-validate portfolio-prepare \
      --strategy short-term-reversal-v1 \
      --research-manifest "$RESEARCH_MANIFEST" \
      --manifest "$manifest" \
      --momentum-top-k "$top_k" \
      --start "$start" --end "$end" \
      --workdir "$ROOT"
    run_logged env GOMEMLIMIT=512MiB GOGC=20 "$BIN" research-validate portfolio-run \
      --manifest "$manifest" \
      --candle-dir "$DATA_DIR" \
      --output "$report" \
      --workdir "$ROOT"
  done
done

jq -s '[.[] | {
  experiment_id, range, strategy, candidate,
  base: {net_return: .base.net_return, max_drawdown: .base.max_drawdown, trade_count: .base.trade_count, average_exposure: .base.average_exposure, max_profit_contribution_percent: .base.max_profit_contribution_percent},
  stress: {net_return: .stress.net_return, max_drawdown: .stress.max_drawdown},
  benchmarks: {btc: .benchmarks.btc_buy_and_hold.net_return, equal_weight: .benchmarks.equal_weight_buy_and_hold.net_return},
  decision
}]' "$RUN_ROOT"/short-term-reversal-*/*/report.json >"$RUN_ROOT/.summary.json.tmp"
mv "$RUN_ROOT/.summary.json.tmp" "$RUN_ROOT/summary.json"

echo
echo "Short-term reversal walk-forward complete: $RUN_ROOT"
echo "Summary: $RUN_ROOT/summary.json"

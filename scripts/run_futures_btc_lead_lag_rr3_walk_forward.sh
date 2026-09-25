#!/usr/bin/env bash
# Time-split robustness check for all five diagnostic-selected BTC-down shorts.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
BIN="${BIN:-$ROOT/bin/cli}"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-futures-v1}"
SYMBOLS="${SYMBOLS:-OPUSDT,INJUSDT,UNIUSDT,TIAUSDT,APTUSDT}"
REVISION="$(git rev-parse --short=12 HEAD)"
RUN_ROOT="${RUN_ROOT:-$DATA_DIR/diagnostics/btc-lead-lag-rr3-walk-forward-${REVISION}}"

[[ -x "$BIN" ]] || { echo "ERROR: build bin/cli first (make build-cli)." >&2; exit 1; }
mkdir -p "$RUN_ROOT"

# These pre-holdout windows measure time-split robustness only. The symbols
# were discovered using the full earlier diagnostic, so this cannot promote a
# trading candidate without a later unseen period.
windows=(
  "fold-001 2025-08-06 2025-11-04"
  "fold-002 2025-11-04 2026-02-02"
  "fold-003 2026-02-02 2026-05-03"
)

for window in "${windows[@]}"; do
  read -r name start end <<<"$window"
  "$BIN" research-validate btc-lead-lag-rr3 \
    --candle-dir "$DATA_DIR" \
    --symbols "$SYMBOLS" \
    --start "$start" \
    --end "$end" \
    --output "$RUN_ROOT/$name.json"
done

jq -s 'to_entries | [ .[] as $fold | $fold.value.symbols[] | {
  fold: ("fold-" + (($fold.key + 1) | tostring)),
  symbol,
  signals,
  trades: .metrics.closed_trade_count,
  win_rate: .metrics.trade_win_rate,
  net_return: .metrics.net_return,
  profit_factor: .metrics.profit_factor,
  max_drawdown: .metrics.max_drawdown
}]' "$RUN_ROOT"/fold-*.json >"$RUN_ROOT/summary.json"

echo "BTC lead-lag RR3 all-symbol walk-forward complete: $RUN_ROOT/summary.json"

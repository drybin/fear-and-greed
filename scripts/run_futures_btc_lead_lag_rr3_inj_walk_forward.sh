#!/usr/bin/env bash
# Time-split robustness check for the frozen INJ-only BTC-down RR3 rule.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
BIN="${BIN:-$ROOT/bin/cli}"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-futures-v1}"
REVISION="$(git rev-parse --short=12 HEAD)"
RUN_ROOT="${RUN_ROOT:-$DATA_DIR/diagnostics/btc-lead-lag-rr3-inj-walk-forward-${REVISION}}"

[[ -x "$BIN" ]] || { echo "ERROR: build bin/cli first (make build-cli)." >&2; exit 1; }
mkdir -p "$RUN_ROOT"

# These are protocol-v2 development test windows. The prior locked holdout is
# deliberately excluded because it has already been inspected diagnostically.
windows=(
  "fold-001 2025-08-06 2025-11-04"
  "fold-002 2025-11-04 2026-02-02"
  "fold-003 2026-02-02 2026-05-03"
)

for window in "${windows[@]}"; do
  read -r name start end <<<"$window"
  output="$RUN_ROOT/$name.json"
  "$BIN" research-validate btc-lead-lag-rr3 \
    --candle-dir "$DATA_DIR" \
    --symbols INJUSDT \
    --start "$start" \
    --end "$end" \
    --output "$output"
done

jq -s '[.[] | .symbols[0] | {
  symbol,
  signals,
  trades: .metrics.closed_trade_count,
  win_rate: .metrics.trade_win_rate,
  net_return: .metrics.net_return,
  profit_factor: .metrics.profit_factor,
  max_drawdown: .metrics.max_drawdown
}]' "$RUN_ROOT"/fold-*.json >"$RUN_ROOT/summary.json"

echo "INJ BTC lead-lag RR3 walk-forward complete: $RUN_ROOT/summary.json"

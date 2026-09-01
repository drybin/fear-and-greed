#!/usr/bin/env bash
# Evaluate one frozen breadth + trend-pullback candidate on disjoint pre-holdout windows.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

: "${RESEARCH_MANIFEST:?set RESEARCH_MANIFEST to the source protocol-v2 manifest}"

REVISION="$(git rev-parse --short=12 HEAD)"
DATA_DIR="${DATA_DIR:-$ROOT/data/research-v2}"
RUN_ROOT="${RUN_ROOT:-$DATA_DIR/portfolio-runs/breadth-pullback-walk-forward-$REVISION}"

# These windows cover only pre-holdout development data. Do not add the locked holdout here.
WINDOWS=(
  "01-2025-02-01_2025-05-03:2025-02-01:2025-05-03"
  "02-2025-05-03_2025-08-06:2025-05-03:2025-08-06"
  "03-2025-08-06_2025-11-04:2025-08-06:2025-11-04"
  "04-2025-11-04_2026-02-03:2025-11-04:2026-02-03"
  "05-2026-02-03_2026-05-03:2026-02-03:2026-05-03"
)

for window in "${WINDOWS[@]}"; do
  IFS=: read -r label start end <<<"$window"
  START="$start" END="$end" \
  ENTRY_MODE=trend-pullback \
  REGIME_MODES=breadth \
  RUN_ROOT="$RUN_ROOT/$label" \
  SKIP_VERIFY="${SKIP_VERIFY:-0}" \
  "$ROOT/scripts/run_portfolio_regime_ablation.sh"
  SKIP_VERIFY=1
done

jq -s '[.[] | {
  experiment_id,
  range,
  strategy,
  candidate,
  base: {
    net_return: .base.net_return,
    max_drawdown: .base.max_drawdown,
    trade_count: .base.trade_count,
    average_exposure: .base.average_exposure,
    max_profit_contribution_percent: .base.max_profit_contribution_percent
  },
  stress: {
    net_return: .stress.net_return,
    max_drawdown: .stress.max_drawdown
  },
  benchmarks: {
    btc: .benchmarks.btc_buy_and_hold.net_return,
    equal_weight: .benchmarks.equal_weight_buy_and_hold.net_return
  },
  decision
}]' "$RUN_ROOT"/*/breadth/report.json >"$RUN_ROOT/.summary.json.tmp"
mv "$RUN_ROOT/.summary.json.tmp" "$RUN_ROOT/summary.json"

echo
echo "Walk-forward complete: $RUN_ROOT"
echo "Summary: $RUN_ROOT/summary.json"

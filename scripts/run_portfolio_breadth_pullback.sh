#!/usr/bin/env bash
# Run the one frozen breadth + trend-pullback portfolio diagnostic.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REVISION="$(git -C "$ROOT" rev-parse --short=12 HEAD)"

ENTRY_MODE=trend-pullback \
REGIME_MODES=breadth \
RUN_ROOT="${RUN_ROOT:-$ROOT/data/research-v2/portfolio-runs/breadth-pullback-$REVISION}" \
exec "$ROOT/scripts/run_portfolio_regime_ablation.sh"

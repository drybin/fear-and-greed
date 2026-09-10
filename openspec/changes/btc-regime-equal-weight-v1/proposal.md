## Why

All portfolio candidates tested so far add a cross-sectional selection rule,
yet none has produced stable positive development performance. Before adding
another asset-selection signal, establish whether simple BTC market timing can
make the frozen diversified spot basket deployable after realistic costs.

## What Changes

- Add `btc-regime-equal-weight-v1`, a single portfolio baseline with no
  per-asset ranking or selection.
- On the first Monday of each month, hold all 50 frozen universe symbols at
  equal target weights only when completed BTC EMA-50 is above completed
  BTC EMA-200; otherwise hold USDT.
- Evaluate the fixed baseline in the five development windows without opening
  holdout.

## Non-Goals

- No adjustment to prior strategy results, no parameter grid, no per-asset
  stop, leverage, short positions, live trading, or locked-holdout access.

## Why

Cross-sectional momentum and its defensive regime variant were rejected before
holdout. A distinct portfolio hypothesis should test whether selecting stable
trends, rather than recent return leaders, produces a more durable long-only
return stream after costs.

## What Changes

- Add `low-volatility-trend-v1`, an independent weekly portfolio strategy.
- Select only symbols with a positive completed 20-day return and a completed
  close above their own EMA-200, then rank them by lowest completed 30-day
  realized volatility.
- Evaluate fixed equal-weight `top-5` and `top-10` candidates in five
  pre-holdout windows.

## Non-Goals

- No alteration of previous momentum, defensive momentum, or relative-strength
  manifests and reports.
- No global BTC/breadth filter, parameter sweep, leverage, short positions,
  live trading, selection, or locked-holdout access.

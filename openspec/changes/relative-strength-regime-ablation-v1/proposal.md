## Why

The first portfolio-native relative-strength diagnostic protected capital in a severe market decline but lost 3.37% versus cash. Its BTC EMA200 and positive-breadth filters jointly enabled allocations in only eight of 38 rebalance weeks. We need to measure which filter creates the defensive behavior before changing the ranking, stops, costs, or portfolio limits.

## What Changes

- Add a frozen `regime_mode` to the relative-strength portfolio manifest.
- Evaluate exactly four modes: `both`, `btc-ema`, `breadth`, and `none`.
- Preserve the same source manifest, universe, date range, 90-day return, 30-day volatility, top-5, rank-10 retention, ATR stop, costs, limits, and decision gates.
- Add one sequential CLI workflow that writes a separate immutable manifest and report for each mode.

## Non-Goals

- No parameter tuning, no changes to the ranking formula, no new data download, and no holdout opening.
- This is diagnostic research only; no result receives `portfolio-pass`.

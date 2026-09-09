## Why

Six standalone intraday and multi-timeframe hypotheses were rejected before
holdout: each remained negative after the fixed execution costs. The next
experiment must test a different source of return rather than retuning another
single-symbol entry pattern.

## What Changes

- Add `slow-cross-sectional-momentum-v1`, a portfolio-native weekly rotation
  candidate using only completed daily returns.
- Evaluate four frozen candidates: 60-day or 90-day momentum, each selecting
  top-5 or top-10 positive-return symbols at equal weights.
- Rebalance the complete target portfolio at each Monday open, hold cash when
  fewer than one positive-momentum symbol exists, and run all five
  non-overlapping development windows without opening holdout.

## Non-Goals

- No modification of existing relative-strength, breadth, or pullback results.
- No BTC/breadth regime filter, volatility-normalized score, ATR stop, short,
  leverage, live trading, or holdout access.

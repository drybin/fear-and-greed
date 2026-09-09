## Why

Trend-following portfolio experiments produced isolated strong windows but no
durable result. A separate hypothesis should test whether short-term losers can
mean-revert while their longer-term individual trend remains positive.

## What Changes

- Add `short-term-reversal-v1`, a weekly portfolio-native contrarian strategy.
- Select symbols above their own completed EMA-200 with a strictly negative
  completed five-day return, then rank the most negative returns first.
- Test equal-weight `top-5` and `top-10` candidates across five fixed
  pre-holdout windows.

## Non-Goals

- No changes to prior momentum, low-volatility, or standalone mean-reversion
  artifacts.
- No parameter sweep, BTC/breadth filter, intraday confirmation, leverage,
  shorting, live trading, selection, or locked-holdout access.

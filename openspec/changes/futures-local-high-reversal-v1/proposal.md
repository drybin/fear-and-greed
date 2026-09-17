# Futures Local High Reversal v1

## Goal

Evaluate the short-side counterpart of the rejected spot local-low reversal in
an independent Binance USD-M perpetual research path. It must not alter prior
spot manifests, data, or reported conclusions.

## Hypothesis

A completed one-hour candle that makes a new high over the preceding 20
completed hours may mean-revert. Sell short at the next hour open, use that
high as stop, take half at 1R and the rest at 2R, and close after 48 hours.

## Data And Costs

- Frozen current top-50 USDT symbols, Binance USD-M perpetuals only.
- Separate `*_futures.csv` hourly kline files and `*_futures_funding.csv`
  funding files.
- Every futures manifest fingerprints both files per symbol.
- Positive funding is paid by longs and received by shorts; settlement applies
  only to a position already open at the funding timestamp.
- Commission and adverse slippage use the normal protocol-v2 base/stress
  profiles.

## Non-Goals

- No leverage optimisation, liquidation model, cross-margin portfolio, or
  mixed long/short book in this change.
- No modification of historical spot runs.

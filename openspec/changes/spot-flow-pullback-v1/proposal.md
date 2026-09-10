# Spot Flow Pullback v1

## Why

All recently tested price-only candidates failed development gates. The existing Binance Spot CSV contract already stores quote volume, trade count, and taker-buy volumes, but research candles previously discarded them.

## Change

Preserve and aggregate the existing spot-flow columns, then add one isolated protocol-v2 candidate. It enters a 1h recovery only after a three-hour pullback in a causal rising 4h EMA200 trend when taker-buy quote share is at least 55% and quote volume is at least the preceding 20h average.

The candidate has no parameter sweep. It uses the signal low as stop, 1R/2R exits, and a 48-hour time exit. Missing or invalid spot-flow data is a hard strategy error, not silently interpreted as zero.

## Scope

The research uses the existing frozen top-50 Spot CSV files, cutoff, three walk-forward folds, costs, controls, stress run, freeze, and review. It does not fetch futures data, funding, order-book depth, or open the final holdout.

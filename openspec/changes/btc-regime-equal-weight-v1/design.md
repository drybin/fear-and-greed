## Hypothesis

A broad, diversified crypto spot basket may be deployable only during a
long-term BTC bull regime. This experiment isolates that market-timing premise
by removing all cross-sectional selection from the portfolio.

## Frozen Rules

- Universe: exactly the frozen current top-50 Binance Spot USDT symbols from
  the source manifest; all checksum-verified CSVs are used.
- Decision cadence: the first Monday of each calendar month. This is the first
  Monday whose day-of-month is from 1 through 7.
- Information set: the decision at that open uses only BTC daily candles
  completed strictly before the fill date.
- Regime: calculate EMA-50 and EMA-200 from the latest completed BTC bars.
  The regime is on only when EMA-50 is strictly above EMA-200.
- Allocation: when on, replace the entire portfolio with all 50 symbols at
  2% target weights. When off, close all positions and retain USDT.
- Execution: existing next-open fills, shared cash, commission, slippage,
  stress profile, immutable manifests, and benchmarks apply unchanged.
- There is no asset rank, per-symbol trend filter, price stop, or global
  parameter sweep.

## Evaluation

The sole candidate runs in five fixed pre-holdout windows:

1. `2025-02-01` to `2025-05-03`
2. `2025-05-03` to `2025-08-06`
3. `2025-08-06` to `2025-11-04`
4. `2025-11-04` to `2026-02-03`
5. `2026-02-03` to `2026-05-03`

The workflow writes five immutable manifest/report pairs and a summary. The
locked holdout beginning `2026-05-03` remains structurally unavailable.

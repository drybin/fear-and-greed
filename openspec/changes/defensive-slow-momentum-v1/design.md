## Hypothesis

Slow cross-sectional momentum may be viable only while the broad market is
healthy. This experiment tests whether a daily causal transition to cash avoids
the regime-change losses observed in the plain weekly strategy.

## Frozen Rules

- Universe and execution: frozen Binance Spot USDT cohort, checksum-verified
  candle inputs, existing next-open fills, base costs and stress costs.
- Ranking: on Mondays, rank all valid symbols by completed raw 60-day or
  90-day return; lexical symbol order breaks ties.
- Candidates: `60d/top5`, `60d/top10`, `90d/top5`, `90d/top10`.
- Allocation: full replacement at the Monday open with equal target weights of
  20% for top-5 and 10% for top-10. There is no intraweek price stop.
- BTC guard: at each daily open, compare the prior completed BTC close with
  its EMA-200 calculated from the last 200 completed BTC daily bars. The guard
  passes only when the close is strictly above the EMA.
- Breadth guard: at the same daily open, compute the fraction of all symbols
  with a valid completed lookback return that is strictly positive. The guard
  passes only at 50% or above.
- Daily risk-off: if either guard fails, close every holding at that open and
  remain in USDT. A healthy non-Monday does not create an entry event.
- Re-entry: a new allocation is possible only on a later Monday for which both
  guards pass. If fewer than one positive-return symbol exists then, cash is
  retained.
- Causality: neither the guard nor the ranking reads the fill-day candle. A
  close occurring on Monday can therefore cause a cash exit no earlier than
  Tuesday's open.

## Evaluation

Run every candidate in each fixed pre-holdout window:

1. `2025-02-01` to `2025-05-03`
2. `2025-05-03` to `2025-08-06`
3. `2025-08-06` to `2025-11-04`
4. `2025-11-04` to `2026-02-03`
5. `2026-02-03` to `2026-05-03`

The workflow creates twenty immutable manifest/report pairs and one summary.
The locked holdout from `2026-05-03` remains inaccessible. Reports are
diagnostic only; a selection or holdout decision requires a separate change.

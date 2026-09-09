## Hypothesis

Cross-sectional leadership may persist for weeks even when individual intraday
entry patterns do not survive costs. Ranking only by completed 60-day or 90-day
return tests this directly, without inheriting the prior risk-adjusted
relative-strength score or its regime filters.

## Frozen Rules

- Universe: the frozen spot USDT cohort and checksum-verified source CSVs.
- Decision cadence: each Monday; the complete prior target set is closed at
  the daily open before new positions are opened at that same open.
- Information set: every score uses bars strictly before that Monday. The
  fill-day candle is excluded from ranking.
- Eligibility: a symbol must have a strictly positive completed return over
  its 60-day or 90-day lookback.
- Ranking: descending raw return; lexical symbol order breaks ties.
- Candidates: `60d/top5`, `60d/top10`, `90d/top5`, `90d/top10`.
- Allocation: equal target weights, respectively 20% or 10% of pre-entry
  equity. The strategy has no intraweek stop; membership is reassessed only at
  the next weekly rebalance.
- Cash: if no eligible positive-return symbol exists, all positions are closed
  and no new position is opened.
- Execution and costs: existing portfolio engine next-open fills, commission,
  slippage, cash constraints, immutable manifest identity, stress profile and
  benchmarks remain unchanged.

## Evaluation

Every candidate runs in each of five fixed half-open development windows:

1. `2025-02-01` to `2025-05-03`
2. `2025-05-03` to `2025-08-06`
3. `2025-08-06` to `2025-11-04`
4. `2025-11-04` to `2026-02-03`
5. `2026-02-03` to `2026-05-03`

The locked holdout beginning `2026-05-03` is structurally unavailable. The
workflow writes one immutable manifest and report per candidate/window plus a
summary that only aggregates written metrics; it does not select a winner or
open holdout. Any follow-up selection requires a new explicit OpenSpec change.

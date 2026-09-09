## Hypothesis

A coin can experience a short-term pullback without losing its longer-term
trend. Buying the deepest completed weekly losers that remain above EMA-200
may capture a one-week rebound. This is a cross-sectional portfolio test, not
the earlier per-symbol RSI or Bollinger entry logic.

## Frozen Rules

- Universe, source CSV checksums, costs, stress costs, benchmarks, and shared
  capital engine are unchanged.
- Each Monday open uses only daily candles completed strictly before that open.
  The fill-day candle cannot affect eligibility or rank.
- Eligibility requires: latest completed close strictly above the symbol's
  EMA-200, and completed five-day return strictly below zero.
- Ranking is ascending completed five-day return: the most negative return
  ranks first; lexical symbol order breaks exact ties.
- Candidates: `top-5` at 20% target weight each and `top-10` at 10% each.
  The entire basket is closed and replaced at every Monday open, creating a
  fixed one-week maximum holding period.
- If no eligible loser exists, all positions are closed and capital remains in
  USDT. There is no global market filter or intraday stop.

## Evaluation

Both candidates run only in these five development windows:

1. `2025-02-01` to `2025-05-03`
2. `2025-05-03` to `2025-08-06`
3. `2025-08-06` to `2025-11-04`
4. `2025-11-04` to `2026-02-03`
5. `2026-02-03` to `2026-05-03`

The runner creates ten immutable manifests/reports and a summary. It does not
open the locked holdout beginning `2026-05-03`. Any subsequent selection or
holdout run requires a distinct explicit OpenSpec change.

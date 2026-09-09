## Hypothesis

Assets that are already in their own sustained uptrend but exhibit relatively
low realized volatility may produce a less fragile portfolio than selecting the
highest recent return. This is a quality-of-trend hypothesis, not a
cross-sectional momentum score.

## Frozen Rules

- Universe, price data, costs, benchmarks, immutable manifests, and shared
  capital engine remain unchanged.
- At each Monday open, use only daily bars completed strictly before that
  open. The fill-day candle never influences eligibility, volatility, or rank.
- A symbol is eligible only when its latest completed close is strictly above
  its EMA-200 and its completed 20-day return is strictly positive.
- Rank eligible symbols by ascending 30-day standard deviation of daily log
  returns. Higher 20-day return, then lexical symbol order, break ties.
- Candidates: equal-weight `top-5` (20% per target) and `top-10` (10% per
  target). The full target basket is replaced each Monday.
- Cash fallback: if no symbol is eligible, close all holdings and retain cash.
- There is no global regime filter or intraweek stop. This isolates the
  per-symbol low-volatility trend hypothesis from rejected market-guard logic.

## Evaluation

Run both candidates in these fixed development windows only:

1. `2025-02-01` to `2025-05-03`
2. `2025-05-03` to `2025-08-06`
3. `2025-08-06` to `2025-11-04`
4. `2025-11-04` to `2026-02-03`
5. `2026-02-03` to `2026-05-03`

The runner writes ten immutable manifest/report pairs plus a summary. It does
not open the locked holdout beginning `2026-05-03`; a new explicit change is
required before any holdout decision.

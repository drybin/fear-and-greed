## Why

The breadth + trend-pullback diagnostic improved one development interval but
produced only four trades and concentrated nearly all positive PnL in BNB. One
interval cannot establish that the entry rule is robust.

## What Changes

- Freeze five chronological, disjoint pre-holdout evaluation windows.
- Record an optional evaluation range in each immutable portfolio manifest.
- Reject ranges that fall outside the source manifest's development horizon or
  touch its locked holdout.
- Produce a compact immutable-equivalent summary from the five reports.

## Non-Goals

- No change to the relative-strength ranking, breadth filter, entry rule,
  costs, risk, gates, universe, or benchmarks.
- No opening of the source locked holdout.

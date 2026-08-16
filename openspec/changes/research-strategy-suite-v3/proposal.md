## Why

The frozen protocol-v2 development study rejected all four core candidates
without opening holdout. The next experiment must test fresh, bounded
hypotheses rather than retune rejected parameters or inspect the locked data.

## What Changes

- Add the independent `research-v3` suite with three spot-long candidates:
  volatility-compression breakout v2, trend mean reversion v1, and daily low
  zone v1.
- Daily low zone v1 forms today's zone from yesterday's low and the first
  earlier daily low strictly below it. A separate green candle must close back
  above yesterday's low after a non-breaching zone touch; the earlier low is
  the stop, yesterday's high is a full-position target, and the position exits
  at the open following one additional calendar day if neither price exit
  occurred.
- Keep the existing core-v2 suite and all frozen v2 artifacts unchanged.
- Add explicit `prepare --suite research-v3`, producing a distinct manifest and
  experiment identity from the same candle cohort.
- Defer cross-sectional relative strength to the portfolio change because it is
  not a standalone per-symbol signal.

## Impact

- Extends candidate registration, manifest suite validation, and in-process
  research evaluation.
- Does not alter execution, sizing, costs, controls, fold construction, gates,
  or the unopened v2 holdout.

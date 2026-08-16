## Why

The existing batch study ranks promising strategies, but per-symbol best-of-sweep selection, overlapping periods, realized-only returns, and frictionless fills prevent an honest out-of-sample conclusion. The first delivery must correct that research foundation and re-evaluate the four existing candidates before expanding strategy or portfolio scope.

## What Changes

- Add immutable experiment manifests, input fingerprints, chronological folds, locked holdout, and reproducible artifacts.
- Add causal next-bar fills, commission, slippage, gap handling, mark-to-market equity, and normalized risk sizing.
- Select one strategy-level parameter set on training data and apply it unchanged to all test symbols.
- Add cash, buy-and-hold, BTC, EMA200, and frequency-matched random controls.
- Adapt and evaluate only `fib-pullback-trend-v1`, `nr7-trend-breakout-v1`, `volatility-compression-breakout-v1`, and `breakout-retest-long-v2`.
- Add one-command software verification and resumable development walk-forward orchestration.
- Enforce a separate freeze and one-time final holdout invocation.
- Classify each candidate as `research-pass`, `observe`, or `reject` from standalone out-of-sample evidence.
- Explicitly defer new strategy hypotheses, shared-capital portfolio evaluation, relative strength, point-in-time universe reconstruction, and advanced statistical hardening to separate changes.

## Capabilities

### New Capabilities

- `realistic-backtest-engine`: Causal fills, costs, mark-to-market accounting, risk sizing, and execution auditability for standalone strategy research.
- `research-validation-protocol`: Immutable manifests, frozen-current-cohort eligibility, train/test separation, global parameter selection, walk-forward evaluation, holdout isolation, reporting, and research decision gates.
- `core-strategy-validation`: Controls and protocol-v2 adapters for the four candidates already identified by exploratory screening.

### Modified Capabilities

None. This repository had no archived OpenSpec capabilities before this change.

## Impact

- Affects standalone simulation, scan orchestration, and report generation under `internal/strategy`, `internal/app/cli/usecase`, and `internal/infrastructure/scanreport`.
- Preserves legacy commands and reports as exploratory artifacts while adding separate protocol-v2 workflows.
- Does not add new strategy ideas, shared-capital portfolio simulation, live trading, or runtime coupling to `fear-and-greed-online`.
- Produces a smaller first milestone: a trustworthy answer about the four existing candidates.

## Follow-up Changes

1. `research-strategy-expansion-v2` adds six standalone hypotheses after this core is complete.
2. `research-portfolio-evaluation-v2` adds shared capital, portfolio constraints, and cross-sectional relative strength.
3. `research-statistical-hardening-v2` adds point-in-time universes and advanced multiple-testing and uncertainty analysis.

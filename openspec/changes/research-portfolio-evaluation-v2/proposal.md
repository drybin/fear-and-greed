## Why

Standalone research uses a separate account per symbol and cannot estimate simultaneous capital demand, correlated drawdown, or deployable portfolio performance. Strategies that earn `research-pass` need a distinct shared-capital evaluation before they can be considered product candidates.

## What Changes

- Add chronological multi-symbol event processing with one cash balance and mark-to-market equity curve.
- Add per-trade risk, per-position notional, concurrent-position, aggregate-risk, and cash constraints.
- Add deterministic ranking and rejection reasons for simultaneous signals.
- Add cash, BTC, and equal-weight portfolio benchmarks.
- Add cross-sectional `relative-strength-long-v1` as a portfolio-native strategy.
- Produce `portfolio-pass / observe / reject` decisions without changing standalone research results.

## Capabilities

### New Capabilities

- `portfolio-evaluation`: Shared-capital simulation, portfolio constraints, relative strength, benchmarks, and portfolio decision gates.

### Modified Capabilities

None.

## Impact

- Depends on `research-validation-v2`; it consumes only frozen `research-pass` and selected `observe` artifacts.
- May also consume strategies from `research-strategy-expansion-v2` when that change is complete.
- Does not alter standalone trade generation or reopen standalone holdouts.

## Dependency

`research-validation-v2` MUST be complete. `research-strategy-expansion-v2` is recommended before the first full comparison but is not required to build the portfolio engine.

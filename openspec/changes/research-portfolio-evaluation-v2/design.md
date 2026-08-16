## Context

Standalone results intentionally isolate signal quality, but averaging those accounts ignores capital contention and correlated signals. Portfolio evaluation consumes immutable signal artifacts and applies allocation decisions without regenerating or retuning them.

## Goals / Non-Goals

**Goals:** shared capital, causal event order, deterministic capacity handling, portfolio risk metrics, aligned benchmarks, and relative strength.

**Non-Goals:** changing strategy signals, reopening standalone holdout, live orders, leverage, shorts, or point-in-time universe reconstruction.

## Decisions

### 1. Portfolio consumes frozen signal artifacts

The engine references source experiment, strategy version, fold, parameters, and signal checksum. It does not call training selection or mutate strategy behavior.

### 2. Primary constraints are explicit

Initial defaults are 1% equity risk per trade, 20% maximum position notional, five concurrent positions, and 5% aggregate initial open risk. All values are manifest-frozen.

### 3. Events are processed chronologically

At one timestamp, exits execute before valuation and new entries. Simultaneous entries are ranked by frozen strategy score, relative strength, and symbol lexical order. Every rejected signal remains auditable.

### 4. Relative strength is portfolio-native

`relative-strength-long-v1` ranks only currently eligible symbols from completed pre-rebalance candles and emits target holdings rather than independent all-in signals.

### 5. Portfolio decision is separate

`portfolio-pass` requires frozen return, drawdown, benchmark, concentration, stress-cost, and capacity gates. Failure does not rewrite the earlier standalone decision.

## Risks / Trade-offs

- **[Priority influences allocation]** → Freeze and report every ranking input.
- **[Correlated assets amplify loss]** → Enforce aggregate risk and report concentration.
- **[Frozen current universe bias remains]** → Preserve core warning until statistical hardening.
- **[Turnover can dominate relative strength]** → Apply identical costs and stress profiles.

## Migration Plan

1. Define portfolio manifest and frozen-signal input contracts.
2. Implement event stream, shared accounting, constraints, and audit trail.
3. Add benchmarks and relative strength.
4. Run portfolio development and final evaluation without reopening standalone holdout.

## Open Questions

- Should `observe` strategies enter the primary portfolio or a separate diagnostic portfolio?

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

The frozen diagnostic candidate is `rs-90d-vol30-top5`:

- aggregate minute inputs to UTC daily OHLCV;
- rebalance on Monday's daily open;
- calculate 90-day close-to-close return using only bars completed before that open;
- divide return by the sample standard deviation of the latest 30 daily log returns;
- resolve equal scores by raw return and then symbol order;
- require BTC's previous close above its 200-day EMA and at least 50% of scored symbols to have positive 90-day return;
- target ranks 1-5 and retain an existing holding until it falls below rank 10;
- size from a two-ATR initial stop, 1% equity risk, and 20% notional cap;
- turn the regime off and exit all holdings when either market filter fails.

The fill-day candle never participates in its own rank, regime, breadth, ATR, or EMA calculation. Stops are checked only after the open-time rebalance. A missing valuation bar carries the last completed close but cannot create an entry or exit fill.

### 5. The first relative-strength run is diagnostic

No completed standalone candidate currently has `research-pass`. The first runnable experiment therefore sets `diagnostic=true` and evaluates the portfolio-native relative-strength strategy without promoting failed standalone strategies. A diagnostic experiment may return only `observe` or `reject`; it cannot return `portfolio-pass`.

Primary portfolio evaluation remains stricter: it requires immutable `research-pass` artifact references and validates every artifact checksum before reading market data.

### 6. Portfolio decision is separate

`portfolio-pass` requires frozen return, drawdown, benchmark, concentration, stress-cost, and capacity gates. Failure does not rewrite the earlier standalone decision.

The frozen gates for this candidate are non-negative net return, drawdown no greater than 25%, return no worse than five percentage points below BTC or equal weight, no single profitable trade contributing more than 40% of total positive trade PnL, and positive return under the stress-cost profile.

### 7. Artifacts and execution are immutable

`portfolio-prepare` derives an identity-addressed manifest from an existing protocol-v2 manifest. It freezes source revision, source manifest identity, full candle fingerprints, evaluation range, costs, limits, candidate parameters, and gates.

`portfolio-run` requires the exact clean Git revision, rechecks every complete CSV checksum before loading the bounded warmup/evaluation range, and writes one immutable JSON report. An identical rerun reuses byte-identical output; a different result cannot overwrite it.

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

## Implementation boundary

This change now contains a runnable diagnostic relative-strength path and the generic shared-capital engine. Importing standalone signal event streams into a primary portfolio remains open until at least one immutable source artifact receives `research-pass`; the manifest and checksum contracts are already enforced, but the first experiment does not pretend failed signals are eligible.

## Resolved Questions

- `observe` and rejected strategies never enter a primary portfolio. They may be evaluated only in a manifest explicitly frozen as diagnostic.

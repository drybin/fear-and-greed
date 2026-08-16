## Context

The latest exploratory batch study found the same leading structure strategies in two current CoinMarketCap cohorts. However, each `(strategy, symbol, period)` selected its own best sweep result, `current_year` overlapped the two-year period, open positions were excluded from optimized profit, and fills omitted realistic costs. The current CMC constituents also create survivorship bias when tested backward.

This change builds only the minimum reliable standalone research path and applies it to the four already-screened candidates. It intentionally stops before adding more hypotheses or a shared-capital portfolio.

## Goals / Non-Goals

**Goals:**

- Reproduce an experiment from a versioned manifest and immutable candle fingerprints.
- Model conservative spot execution and include open positions in equity.
- Separate parameter selection from chronological out-of-sample evaluation.
- Use one deployable parameter set per strategy and fold, not one hindsight set per symbol.
- Compare the four existing candidates with simple controls under identical assumptions.
- Freeze candidates before opening the final holdout once.
- Produce `research-pass`, `observe`, or `reject` decisions with complete evidence.

**Non-Goals:**

- Implementing Donchian, EMA pullback, RSI, Bollinger, volume breakout, capitulation, or other new hypotheses.
- Building shared-capital portfolio simulation or cross-sectional relative strength.
- Reconstructing historical point-in-time CMC membership.
- Adding bootstrap confidence intervals, false-discovery controls, or other advanced statistical hardening.
- Building a trading bot, short/futures simulation, or online-app runtime dependency.

## Decisions

### 1. Protocol-v2 runs start from immutable manifests

The manifest records protocol version, source revision, data cutoff, universe snapshot, symbols, candle fingerprints, folds, strategy versions, parameter grids, execution costs, sizing, random seed, and decision gates. Canonical serialization produces the experiment identity. Changed research inputs create a new identity and never overwrite prior results.

### 2. Core uses frozen current cohorts with an explicit claim limit

The initial top-50 and CMC #50-200 snapshots are classified as `frozen-current-cohort`. Reports must state that backward tests describe those frozen watchlists and cannot establish historical top-N performance. Point-in-time membership is deferred to `research-statistical-hardening-v2`.

Primary results require at least 12 months of usable pre-test history and complete strategy warmup. Shorter listings are retained only in a labeled secondary cohort.

### 3. Validation is chronological and nested

The end of the available history is reserved as a locked holdout. Development history is divided into rolling folds with at least nine months training followed by three months testing and a three-month step when data permits. Warmup may precede a fold but contributes no PnL.

At least three development test folds are required for `research-pass`. A shorter run remains exploratory.

Development stores one checksum-protected gzip checkpoint artifact per unit.
Compression and checksum calculation stream directly to the atomic temporary
file and MUST NOT retain a second full compressed artifact in memory.
It does not eagerly duplicate each checkpoint into per-symbol report trees;
expanded reports are materialized only for the bounded final phase. Incomplete
run directories from obsolete source revisions may be removed explicitly,
while frozen and final runs remain immutable.

Freeze validates checkpoint checksums and unit identity as a bounded-memory
stream; it does not decode symbol evidence arrays. A non-evaluating recovery
mode may freeze a completed older-revision development with a newer
orchestrator, but it preserves the original source hash and cannot run
development or final evaluation.

Training-grid checkpoints are summary-only: they retain exact scalar metrics,
final equity, trade/rejection counts, and symbol identity needed for global
train-only selection. Raw trades, equity curves, rejections, and audit events
are retained for test and final units, where decision gates and reports consume
them. This keeps training evidence bounded without changing candidate scores.

### 4. Parameter selection is global and train-only

Every strategy declares at most 30 primary parameter combinations. Each candidate is evaluated across the complete eligible training cohort. A deterministic score uses median symbol performance with drawdown, low-sample, and concentration penalties. The selected candidate is applied unchanged to every eligible test symbol.

Fold-specific retraining is allowed because it models periodic recalibration, but a test or holdout result can never influence its preceding selection.

### 5. Execution uses conservative next-bar semantics

Signals requiring candle close are filled no earlier than the next bar open. The initial base profile uses 10 basis points commission and 5 basis points adverse slippage per fill; stress uses 10 and 15 basis points respectively. Both remain configurable and frozen in the manifest.

The execution engine evaluates candles aggregated to the strategy timeframe
declared in the manifest. Retained 1m Spot data remains the canonical source
for deterministic aggregation, but the engine MUST NOT create minute-level
equity and audit events for a 15m or 1h strategy.

Long gap-through-stop exits fill at the next open. When stop and target are both touched in one aggregate candle and ordering is unknown, the primary profile processes the stop first. Every partial fill receives its own costs.

### 6. Standalone accounts use mark-to-market equity and normalized risk

Each `(strategy, symbol, fold)` simulation has its own account so signal quality can be compared without portfolio allocation effects. It risks 1% of current equity at the initial stop and caps notional at 20% of equity. Open positions are marked to market on every bar and at fold end. Realized cash remains diagnostic only.

Quantities use the protocol's nominal eight-decimal precision. For very
low-priced assets whose position count is large enough that one quantity tick
is smaller than a representable `float64` step, validation accepts only a
bounded ULP-level difference from the rounded value. Material over-exits,
under-reconciled closed trades, zero quantities, and non-finite quantities
remain invalid.

Shared cash and concurrent-position constraints belong to `research-portfolio-evaluation-v2` and are not implied by standalone aggregates.

### 7. Core metrics are deliberately bounded

Core reports include net return, annualized return where meaningful, maximum drawdown, Calmar ratio, expectancy, profit factor, payoff ratio, trade win rate, completed trades, holding time, exposure, turnover, costs, symbol breadth, fold consistency, and contribution concentration.

Advanced bootstrap and multiple-testing statistics are deferred. Raw metrics and every parameter result are retained so the later hardening change can consume them without rerunning or losing evidence.

### 8. Research decisions do not claim portfolio readiness

The default `research-pass` gate requires:

- at least 100 aggregate out-of-sample trades and 20 eligible symbols;
- positive net result in at least 60% of development test folds;
- aggregate out-of-sample profit factor of at least 1.15 after base costs;
- positive aggregate expectancy;
- median per-symbol maximum drawdown no greater than 20%;
- no single symbol contributing more than 25% of positive pooled PnL;
- positive result under stress costs;
- no material collapse in immediately neighboring parameter candidates;
- a locked holdout result consistent with the frozen gates.

`research-pass` means the hypothesis is eligible for later strategy-expansion comparison and portfolio evaluation. It does not authorize live trading or online implementation. `observe` means net-positive but under-sampled or insufficiently robust. `reject` means non-positive expectancy, failed stress costs, or unacceptable risk.

### 9. Core strategy scope is fixed

Only four screened candidates are adapted: fib trend pullback, NR7 trend breakout, volatility compression breakout, and breakout retest v2. Controls are cash, per-asset buy-and-hold, BTC buy-and-hold, EMA200 long/cash, and seeded frequency-matched random entry.

Behavioral fixes create a new strategy version. Execution/accounting differences caused by protocol v2 are documented through zero-cost reconciliation fixtures.

### 10. Workflow has a deliberate final boundary

Before the research phases, `prepare` validates the frozen 50-symbol 1m Spot
dataset, fingerprints every CSV, reads the clean Git revision, and writes the
canonical manifest with the core adapter metadata, grids, schedule, costs,
risk, seed, and gates.

The top-level phases are:

1. `verify`: unit, integration, property, golden, leakage, determinism, and report-schema tests.
2. `development`: controls, training selection, walk-forward tests, cost stress, robustness, and reports; holdout access is prohibited.
3. `freeze`: immutable source revision, strategy versions, parameters, gates, manifest, fingerprints, and development report hash.
4. `final`: one-time holdout evaluation without retraining or gate changes.

Development writes atomic per-strategy and per-fold checkpoints. Resume verifies all source, manifest, input, and artifact hashes before reuse. A convenience workflow stops after freeze and requires explicit final authorization.

Before authorization, `review` streams completed test/control checkpoints into a
compact pre-holdout report. Final records one immutable opening and then writes
one compressed, checksum-protected checkpoint per holdout unit. An interruption
resumes that same opening; it is not treated as permission to create a second
opening or alter any frozen scientific input. Gate aggregation loads one unit at
a time so raw trades, equity, rejections, and audit events do not accumulate in
memory.
The end-to-end operator script runs fetch, verify, prepare, development, and
freeze in order. It can enter final only when the operator supplies an explicit
holdout authorization flag.

## Risks / Trade-offs

- **[Only two years of data]** Few regimes may fit before holdout. → Require three folds for `research-pass`; otherwise report exploratory evidence.
- **[Survivorship bias]** Current cohorts omit historical failures and rank changes. → Restrict claims and defer point-in-time reconstruction.
- **[Conservative fills]** Stop-first and adverse slippage may understate favorable execution. → Keep the conservative profile primary and retain gross diagnostics.
- **[Training overfit remains possible]** Global selection reduces but does not eliminate it. → Bound grids, inspect neighboring candidates, and preserve the holdout.
- **[Standalone result is not deployable portfolio performance]** Separate accounts ignore simultaneous signals. → Use `research-pass`, not `promote`, and require the later portfolio change.
- **[Corrected results differ from legacy reports]** Costs, sizing, MTM, and next-bar execution change PnL. → Preserve and clearly label legacy results.

## Migration Plan

1. Add manifest, identity, eligibility, and versioned report foundations beside legacy workflows.
2. Implement execution, costs, sizing, MTM, and deterministic fixtures.
3. Implement core metrics, folds, global selection, and holdout isolation.
4. Add controls and adapt the four candidates.
5. Add verify/development/freeze/final orchestration with checkpoint resume.
6. Run development, freeze the shortlist, open holdout once, and publish decisions.

Rollback is non-destructive because protocol-v2 commands and output paths remain separate from legacy scans.

## Open Questions

- Should the locked holdout be a fixed three-month interval or 20% once longer data is collected?
- Is 1% standalone risk the preferred permanent baseline, or should a later manifest use 0.5%?

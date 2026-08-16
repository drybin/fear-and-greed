## 1. Protocol boundaries

- [x] 1.1 Define packages for manifest, eligibility, execution, metrics, validation, reporting, and orchestration
- [x] 1.2 Define stable identifiers for experiment, strategy version, parameter candidate, fold, symbol, timeframe, cost profile, and sizing profile
- [x] 1.3 Define enums for phase, run status, cohort status, decision status, and rejection reason
- [x] 1.4 Define UTC and inclusive-start/exclusive-end range conventions
- [x] 1.5 Define deterministic price, quantity, fee, and metric rounding
- [x] 1.6 Define protocol-v2 output paths separately from legacy reports
- [x] 1.7 Define schema-version rules for manifests, checkpoints, and reports
- [x] 1.8 Add contract tests for identifiers, enums, time ranges, serialization, and package boundaries

## 2. Immutable experiment manifest

- [x] 2.1 Add identity, protocol version, cutoff, source revision, and seed fields
- [x] 2.2 Add frozen universe snapshot, exchange, spot market, quote asset, symbol, and exclusion fields
- [x] 2.3 Add strategy version, timeframe, warmup, default parameter, and bounded-grid fields
- [x] 2.4 Add train, test, fold-step, and locked-holdout fields
- [x] 2.5 Add commission, slippage, gap, intrabar, sizing, and standalone-risk fields
- [x] 2.6 Add sample, fold, profitability, drawdown, concentration, stress, stability, and holdout gates
- [x] 2.7 Reject unknown fields and invalid enum values during decoding
- [x] 2.8 Validate chronological ranges and all cross-field constraints
- [x] 2.9 Implement canonical serialization and manifest hashing
- [x] 2.10 Record Git revision and dirty-worktree status
- [x] 2.11 Refuse overwrite when experiment identity differs
- [x] 2.12 Add required-field, invalid-combination, stable-hash, and changed-identity tests

## 3. Data inventory and frozen-cohort eligibility

- [x] 3.1 Inventory each input file's columns, range, row count, and detected interval
- [x] 3.2 Detect unordered, duplicate, missing, malformed, and non-finite OHLC records
- [x] 3.3 Validate volume separately without making it a core-strategy requirement
- [x] 3.4 Fingerprint every candle input
- [x] 3.5 Load dated top-50 and CMC #50-200 frozen snapshot files
- [x] 3.6 Attach the mandatory survivorship warning to frozen-cohort reports
- [x] 3.7 Enforce 12 months of usable pre-test primary history
- [x] 3.8 Enforce strategy-specific warmup without warmup PnL
- [x] 3.9 Report short listings in a secondary cohort with exclusion reasons
- [x] 3.10 Add eligibility fixtures for gaps, duplicates, malformed candles, short history, and insufficient warmup

## 4. Strategy and execution domain

- [x] 4.1 Define core strategy metadata and deterministic registry contracts
- [x] 4.2 Define close-confirmed signal records with source candle, stop, and diagnostics
- [x] 4.3 Define order intent separately from strategy signal
- [x] 4.4 Define entry, partial-exit, and final-exit fill records
- [x] 4.5 Define position state, trade state, and exit reasons
- [x] 4.6 Define equity snapshots with cash, open value, realized, unrealized, costs, and total equity
- [x] 4.7 Define structured signal rejection records
- [x] 4.8 Reject duplicate strategy code/version registrations
- [x] 4.9 Add serialization and invariant tests for every domain record
- [x] 4.10 Add deterministic registry-order tests

## 5. Causal standalone execution

- [x] 5.1 Map close-confirmed signals to the next available candle open
- [x] 5.2 Prevent same-signal-candle fills
- [x] 5.3 Implement and audit the missing-next-interval policy
- [x] 5.4 Apply adverse slippage to long entries and exits
- [x] 5.5 Apply commission independently to every fill
- [x] 5.6 Implement normal stop fills
- [x] 5.7 Implement gap-through-stop fills at the next open
- [x] 5.8 Implement target fills under the primary profile
- [x] 5.9 Process stop before target when one candle touches both
- [x] 5.10 Implement TP1 partial exit followed by breakeven or TP2
- [x] 5.11 Implement declared time exits and end-of-fold handling
- [x] 5.12 Reject invalid prices, quantities, stops, and causal timestamps
- [x] 5.13 Persist all fill inputs, costs, and decisions in an audit trail
- [x] 5.14 Add one deterministic fixture for every fill rule
- [x] 5.15 Add property tests that sold quantity never exceeds bought quantity
- [x] 5.16 Add gross-minus-costs-equals-net reconciliation tests

## 6. Mark-to-market and standalone sizing

- [x] 6.1 Mark every open position to market on each evaluation bar
- [x] 6.2 Include open positions in final fold equity and drawdown
- [x] 6.3 Keep realized cash diagnostic-only for selection
- [x] 6.4 Track cash and position value after every fill
- [x] 6.5 Track equity peaks and underwater percentage
- [x] 6.6 Size quantity from 1% equity risk and stop distance
- [x] 6.7 Cap position notional at 20% of equity
- [x] 6.8 Include expected entry costs in cash sufficiency checks
- [x] 6.9 Reject zero, inverted, and non-finite stop distances
- [x] 6.10 Define deterministic quantity rounding
- [x] 6.11 Reconcile cash plus open value to equity after every event
- [x] 6.12 Add fixtures for winning, losing, partial, open-ended, capped, and invalid-stop trades

## 7. Core metrics and reports

- [x] 7.1 Implement gross return, net return, and duration-qualified annualized return
- [x] 7.2 Implement drawdown series, maximum drawdown, duration, and Calmar ratio
- [x] 7.3 Implement expectancy in currency and R units
- [x] 7.4 Implement profit factor, payoff ratio, and edge-case handling
- [x] 7.5 Implement trade wins, losses, breakevens, and `trade_win_rate`
- [x] 7.6 Implement trade counts and holding-time statistics
- [x] 7.7 Implement exposure, capital utilization, and turnover
- [x] 7.8 Implement total commission and slippage drag
- [x] 7.9 Implement `symbol_win_rate`, breadth, fold consistency, and contribution concentration
- [x] 7.10 Define versioned summary, fold, candidate, trade, fill, equity, rejection, and eligibility schemas
- [x] 7.11 Retain every candidate and fold result with deterministic selection explanation
- [x] 7.12 Write reports atomically and checksum completed artifacts
- [x] 7.13 Add schema validation and golden report fixtures
- [x] 7.14 Add metric fixtures for no trades, all wins, all losses, breakeven, open positions, and unequal histories

## 8. Walk-forward, selection, and research gates

- [x] 8.1 Generate warmup, nine-month train, three-month test, and three-month-step ranges
- [x] 8.2 Reserve holdout before development fold generation
- [x] 8.3 Prevent train/test/holdout overlap and reverse information flow
- [x] 8.4 Require three development test folds for `research-pass`
- [x] 8.5 Label shorter experiments exploratory
- [x] 8.6 Validate no more than 30 unique parameter candidates per strategy
- [x] 8.7 Run every candidate across the complete eligible training cohort
- [x] 8.8 Aggregate training score by median symbol evidence with drawdown, sample, and concentration penalties
- [x] 8.9 Apply deterministic tie-breaking
- [x] 8.10 Select one candidate per strategy and fold
- [x] 8.11 Apply the selected candidate unchanged to every test symbol
- [x] 8.12 Calculate neighboring-candidate sensitivity and cross-fold parameter stability
- [x] 8.13 Implement every frozen sample, fold, PF, expectancy, drawdown, concentration, stress, stability, and holdout gate
- [x] 8.14 Implement `research-pass`, `observe`, and `reject`
- [x] 8.15 Persist each gate threshold, input, result, and explanation
- [x] 8.16 Add poisoned-test leakage tests proving training selection is unchanged
- [x] 8.17 Add poisoned-holdout leakage tests proving development artifacts are byte-identical
- [x] 8.18 Add fixtures for each gate and each final decision

## 9. Controls

- [x] 9.1 Implement cash control
- [x] 9.2 Implement aligned per-asset buy-and-hold
- [x] 9.3 Implement aligned BTC buy-and-hold
- [x] 9.4 Implement causal EMA200 long/cash
- [x] 9.5 Define activity matching for random control
- [x] 9.6 Implement seeded frequency-matched random entry
- [x] 9.7 Apply compatible dates, costs, sizing, and accounting to controls
- [x] 9.8 Add deterministic control fixtures and report tests

## 10. Four candidate adapters

- [x] 10.1 Capture legacy zero-cost signal fixtures for `fib-pullback-trend-v1`
- [x] 10.2 Adapt fib metadata, signals, stops, diagnostics, and bounded grid
- [x] 10.3 Capture legacy zero-cost signal fixtures for `nr7-trend-breakout-v1`
- [x] 10.4 Adapt NR7 metadata, signals, stops, diagnostics, and bounded grid
- [x] 10.5 Capture legacy zero-cost signal fixtures for `volatility-compression-breakout-v1`
- [x] 10.6 Adapt VCB metadata, signals, stops, diagnostics, and bounded grid
- [x] 10.7 Capture legacy zero-cost signal fixtures for `breakout-retest-long-v2`
- [x] 10.8 Adapt BR v2 metadata, signals, stops, diagnostics, and bounded grid
- [x] 10.9 Document every intentional timing, execution, sizing, and accounting difference
- [x] 10.10 Add no-lookahead fixtures for higher/lower-timeframe interaction
- [x] 10.11 Add invalid-stop and insufficient-warmup fixtures for every candidate
- [x] 10.12 Verify adapters contain no test-period or per-symbol parameter selection
- [x] 10.13 Reject deferred strategy codes in core manifests
- [x] 10.14 Add registry and common-contract tests for exactly four candidates

## 11. Verify, development, freeze, and final orchestration

- [x] 11.1 Define CLI contracts for `verify`, `development`, `freeze`, and `final`
- [x] 11.2 Run unit, integration, property, golden, leakage, determinism, and schema suites from `verify`
- [x] 11.3 Print one combined verification summary and reproduction commands for failures
- [x] 11.4 Run development preflight for manifest, revision, fingerprints, eligibility, disk, and output path
- [x] 11.5 Orchestrate controls, four candidates, folds, base costs, stress costs, sensitivity, and reports from one development command
- [x] 11.6 Prohibit development from reading holdout observations or results
- [x] 11.7 Define atomic per-strategy, candidate, fold, and cost-profile checkpoints
- [x] 11.8 Resume only checksum-valid units with matching source, manifest, and data hashes
- [x] 11.9 Report progress, completed units, failures, and remaining units
- [x] 11.10 Freeze source revision, strategy versions, parameters, gates, manifest, fingerprints, and development report hash
- [x] 11.11 Stop automatically after freeze and print but do not execute the final command
- [x] 11.12 Verify the complete frozen bundle before final
- [x] 11.13 Record holdout opening before reading holdout results
- [x] 11.14 Prevent final retraining, parameter changes, gate changes, and a second opening
- [x] 11.15 Add orchestration tests for success, failure, interruption, resume, stale checkpoint, freeze, and forbidden holdout access
- [x] 11.16 Add `prepare` to validate 50 retained 1m Spot CSV files and generate the canonical manifest from actual adapter metadata and fingerprints
- [x] 11.17 Add a resumable end-to-end operator script for fetch, verify, prepare, development, and freeze with explicit opt-in final authorization
- [x] 11.18 Keep per-candle timestamps internal to eligibility so preflight reports remain bounded for multi-year minute data
- [x] 11.19 Add a compact checksum-verified pre-holdout development review
- [x] 11.20 Atomically checkpoint and resume final units under one immutable holdout opening
- [x] 11.21 Bound final memory by loading one result at a time and retaining compressed canonical artifacts
- [x] 11.22 Permit explicit recovery from an orchestration-only upgrade only after a strict Git path audit

## 12. First core research run and acceptance

- [ ] 12.1 Freeze the first data cutoff and cohort snapshot files
- [ ] 12.2 Review and freeze eligibility output and fold boundaries
- [ ] 12.3 Freeze four strategy versions and their bounded grids
- [ ] 12.4 Freeze base/stress costs, standalone sizing, and research gates
- [ ] 12.5 Run and retain the complete verify summary
- [ ] 12.6 Run resumable development for controls and four candidates
- [ ] 12.7 Fix implementation defects only; version behavioral changes under a new experiment identity
- [ ] 12.8 Produce the development shortlist from frozen gates
- [ ] 12.9 Freeze and checksum the candidate bundle
- [ ] 12.10 Explicitly authorize and run holdout exactly once
- [ ] 12.11 Assign `research-pass`, `observe`, and `reject` without post-holdout tuning
- [ ] 12.12 Publish the core protocol-v2 summary, limitations, controls, metrics, and artifact paths
- [x] 12.13 Document manifest creation, assumptions, phases, resume, gates, and holdout policy
- [x] 12.14 Run a fixture quickstart from a clean checkout and verify byte-stable artifacts
- [x] 12.15 Verify legacy workflows remain available or document deliberate incompatibility
- [ ] 12.16 Validate and archive this change before starting dependent implementation changes

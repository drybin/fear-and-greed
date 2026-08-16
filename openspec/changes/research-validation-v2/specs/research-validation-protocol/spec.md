## ADDED Requirements

### Requirement: Immutable experiment manifest
Every protocol-v2 experiment MUST record all reproducibility inputs and candle fingerprints in a strictly validated, canonically hashed manifest.

#### Scenario: Missing research input
- **WHEN** a cutoff, fold, strategy version, grid, cost profile, sizing profile, universe snapshot, gate, or seed is absent
- **THEN** execution fails before simulation

#### Scenario: Changed input
- **WHEN** a manifest or candle fingerprint differs from an existing run
- **THEN** the system creates a new experiment identity and cannot overwrite the prior run

### Requirement: Frozen-cohort eligibility
Core validation SHALL classify its current universe snapshot as `frozen-current-cohort`, display a survivorship warning, and enforce primary-cohort history and quality eligibility.

#### Scenario: Historical claim
- **WHEN** current constituents are tested on earlier candles
- **THEN** the report describes frozen-watchlist behavior and does not claim historical top-N performance

#### Scenario: Short listing
- **WHEN** a symbol lacks 12 months of usable pre-test history
- **THEN** it is excluded from primary aggregates and reported separately

### Requirement: Rolling walk-forward folds
The protocol SHALL use chronological warmup, training, and out-of-sample test ranges without overlap or reverse information flow.

#### Scenario: Standard fold
- **WHEN** sufficient history exists
- **THEN** a fold uses at least nine training months followed by three test months

#### Scenario: Insufficient folds
- **WHEN** fewer than three development test folds fit before holdout
- **THEN** the experiment is exploratory and cannot produce `research-pass`

### Requirement: Train-only global parameter selection
The protocol SHALL select one parameter candidate per strategy from the complete eligible training cohort and apply it unchanged to every test symbol.

#### Scenario: Symbol-specific winner differs
- **WHEN** individual symbols favor different candidates
- **THEN** no symbol receives its own hindsight-selected test parameters

#### Scenario: Test winner differs
- **WHEN** another candidate would win on test data
- **THEN** the train-selected candidate remains the scored result

### Requirement: Locked final holdout
The protocol SHALL reserve a chronological holdout that is inaccessible to development and can be opened once only after freeze.

#### Scenario: Automated preparation and workflow
- **WHEN** the operator starts the end-to-end workflow from a clean source revision and a frozen 50-symbol snapshot
- **THEN** the system retains 1m Spot CSV files, fingerprints them, creates the canonical manifest, verifies the implementation, runs resumable development, freezes the bundle, and stops before holdout unless final authorization is explicit

#### Scenario: Development access attempt
- **WHEN** verify or development requests holdout observations or results
- **THEN** access is rejected

#### Scenario: Final opening
- **WHEN** source revision, versions, parameters, gates, manifest, fingerprints, and development report are frozen
- **THEN** final may evaluate the holdout once without retraining

#### Scenario: Interrupted final resumes the same opening
- **WHEN** final stops after `opened.json` and one or more checksum-valid holdout units were completed
- **THEN** a retry with the identical freeze bundle reuses those units, evaluates only missing units, and does not rewrite the opening record

#### Scenario: Changed final inputs
- **WHEN** a retry presents another bundle, source hash, data hash, unit identity, or damaged artifact
- **THEN** resume fails before reusing or evaluating a holdout unit

#### Scenario: Orchestration-only recovery upgrade
- **WHEN** the final implementation is newer than the frozen evaluator revision
- **THEN** recovery is allowed only by explicit authorization and only when Git proves every intervening path is restricted to approved orchestration, CLI, test, and documentation files

### Requirement: Core metrics and complete retention
Reports SHALL include core return, risk, activity, cost, breadth, consistency, and concentration metrics and retain every parameter and fold result.

#### Scenario: Completed parameter evaluation
- **WHEN** candidate evaluation finishes
- **THEN** winners, losers, metrics, selected-candidate explanation, trades, fills, and equity artifacts are retained

#### Scenario: Win-rate labels
- **WHEN** reports include profitable trades and profitable symbols
- **THEN** they use distinct `trade_win_rate` and `symbol_win_rate` labels

### Requirement: Frozen standalone research gates
The protocol SHALL classify candidates as `research-pass`, `observe`, or `reject` only from manifest-defined standalone out-of-sample gates.

#### Scenario: Research pass
- **WHEN** all sample, fold, profit-factor, expectancy, per-symbol drawdown, concentration, stress-cost, parameter-stability, and holdout gates pass
- **THEN** the strategy is `research-pass` and eligible for later portfolio evaluation

#### Scenario: Positive but incomplete evidence
- **WHEN** net expectancy is positive but sample or robustness requirements fail
- **THEN** the strategy is `observe`

#### Scenario: Failed edge
- **WHEN** expectancy is non-positive, stress costs erase the result, or risk gates fail
- **THEN** the strategy is `reject`

### Requirement: Staged core orchestration
The system SHALL expose deterministic `verify`, `development`, `freeze`, and `final` phases and SHALL stop before final until explicitly authorized.

#### Scenario: One-command verification
- **WHEN** `verify` starts
- **THEN** all technical, determinism, leakage, golden, and schema tests run with one combined result

#### Scenario: One-command development
- **WHEN** `development` starts with a valid manifest
- **THEN** controls and four candidates run across training selection, test folds, cost profiles, and robustness reports without holdout access

#### Scenario: Freeze boundary
- **WHEN** development completes
- **THEN** `freeze` writes an immutable candidate bundle and does not start final automatically

### Requirement: Resumable development
Development SHALL checkpoint completed work atomically and reuse it only when source, manifest, data, and artifact hashes match.

#### Scenario: Interrupted run
- **WHEN** development stops after valid units complete
- **THEN** resume continues incomplete units and reuses checksum-valid completed units

#### Scenario: Stale checkpoint
- **WHEN** any relevant hash differs
- **THEN** resume refuses reuse and requires a new identity or clean execution

### Requirement: Pre-holdout development review
The protocol SHALL produce a compact review from checksum-valid development test and control checkpoints without reading holdout observations.

#### Scenario: Review completed development
- **WHEN** the operator invokes `review` for complete frozen development
- **THEN** the report contains per-fold base/stress evidence, controls, aggregate metrics, parameter stability, neighbor robustness, irreversible blockers, and preliminary development-only gate flags clearly distinguished from final gates that still incorporate holdout

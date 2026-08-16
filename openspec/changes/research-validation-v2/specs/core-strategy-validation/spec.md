## ADDED Requirements

### Requirement: Common core strategy contract
Each core candidate SHALL expose stable code and version, required timeframes, warmup, bounded parameter grid, close-confirmed signals, initial stop, and structured diagnostics.

#### Scenario: Candidate registration
- **WHEN** a core candidate is registered
- **THEN** registration fails unless all required metadata and deterministic signal entry points exist

### Requirement: Four-candidate scope
Core validation SHALL include only `fib-pullback-trend-v1`, `nr7-trend-breakout-v1`, `volatility-compression-breakout-v1`, and `breakout-retest-long-v2`.

#### Scenario: Development run strategy list
- **WHEN** the core development phase builds its strategy plan
- **THEN** it includes exactly the four configured candidates and does not include deferred strategy hypotheses

### Requirement: Legacy reconciliation
Every adapted candidate SHALL have zero-cost fixtures that explain intentional differences between legacy and protocol-v2 signal, execution, sizing, and accounting behavior.

#### Scenario: Changed result under protocol v2
- **WHEN** a protocol-v2 result differs from a legacy report
- **THEN** fixtures identify whether the difference comes from a defect, next-bar execution, costs, sizing, MTM, or an explicitly versioned behavior change

### Requirement: Bounded candidate grids
Each core strategy SHALL declare no more than 30 unique primary parameter candidates before development execution.

#### Scenario: Per-symbol optimization attempt
- **WHEN** a strategy adapter attempts to select parameters from one test symbol
- **THEN** the protocol rejects the result because selection belongs to train-only global orchestration

### Requirement: Core controls
Core validation SHALL provide cash, per-asset buy-and-hold, BTC buy-and-hold, EMA200 long/cash, and frequency-matched random-entry controls under compatible dates and accounting assumptions.

#### Scenario: Candidate comparison
- **WHEN** a candidate fold report is produced
- **THEN** it references the aligned control results for the same scored timestamps

### Requirement: Deferred strategies remain absent
The core registry SHALL NOT implement or execute new trend, mean-reversion, capitulation, portfolio, or relative-strength hypotheses.

#### Scenario: Deferred strategy requested
- **WHEN** a core manifest requests a strategy owned by a follow-up change
- **THEN** validation fails with a scope-specific error

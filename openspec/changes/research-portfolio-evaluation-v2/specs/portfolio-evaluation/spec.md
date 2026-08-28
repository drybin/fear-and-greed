## ADDED Requirements

### Requirement: Frozen signal consumption
The portfolio engine SHALL consume immutable standalone signal artifacts and SHALL NOT retrain or modify their strategy parameters.

#### Scenario: Source mismatch
- **WHEN** source experiment, strategy version, parameter, or checksum differs
- **THEN** portfolio execution fails before allocation

### Requirement: Shared-capital event processing
The engine SHALL merge all market and signal events chronologically while maintaining one cash balance and mark-to-market equity curve.

#### Scenario: Simultaneous entries
- **WHEN** multiple signals share a fill timestamp
- **THEN** they compete for the same pre-entry cash and risk capacity

### Requirement: Portfolio constraints
The engine SHALL enforce manifest-defined trade risk, position notional, position count, aggregate risk, and cash limits.

#### Scenario: Aggregate risk full
- **WHEN** a signal would exceed the configured aggregate initial risk
- **THEN** it is rejected with `aggregate_risk_limit`

### Requirement: Deterministic priority
Capacity-limited simultaneous signals SHALL be ranked deterministically and every ranking input SHALL be retained.

#### Scenario: Equal primary score
- **WHEN** signals tie on strategy score
- **THEN** relative strength and finally symbol order resolve the tie

### Requirement: Portfolio benchmarks
Every portfolio report SHALL compare aligned cash, BTC buy-and-hold, and equal-weight eligible-universe buy-and-hold.

#### Scenario: Benchmark alignment
- **WHEN** a portfolio fold begins
- **THEN** every benchmark uses identical scored timestamps and starting capital

### Requirement: Relative strength
`relative-strength-long-v1` SHALL rank eligible symbols from completed pre-rebalance candles and emit deterministic target holdings.

#### Scenario: Rebalance
- **WHEN** a scheduled rebalance boundary arrives
- **THEN** only then-eligible symbols and prior completed returns enter ranking

#### Scenario: Diagnostic first run
- **WHEN** no immutable standalone artifact has `research-pass`
- **THEN** relative strength may run only with `diagnostic=true` and cannot receive `portfolio-pass`

#### Scenario: Current rebalance candle
- **WHEN** the fill-day candle later changes
- **THEN** its own ranking, regime decision, and opening orders remain unchanged

### Requirement: Portfolio decision
The engine SHALL produce `portfolio-pass`, `observe`, or `reject` from manifest-frozen portfolio gates without changing standalone decisions.

#### Scenario: Portfolio rejection
- **WHEN** correlated drawdown, concentration, benchmark, capacity, or stress-cost gates fail
- **THEN** the portfolio result is rejected while source research decisions remain unchanged

### Requirement: Reproducible portfolio workflow
The workflow SHALL freeze a portfolio manifest before execution, verify the exact clean source revision and all input checksums, and SHALL NOT overwrite a different completed report.

#### Scenario: Existing data
- **WHEN** the portfolio experiment starts from an existing protocol-v2 universe
- **THEN** it reads the fingerprinted local candle files without downloading them again

## ADDED Requirements

### Requirement: Frozen regime ablation
The system SHALL freeze exactly one relative-strength market regime mode in each portfolio manifest.

#### Scenario: Mode identity
- **WHEN** two manifests differ only by regime mode
- **THEN** they have different identities and reports cannot overwrite each other

### Requirement: Four causal filter modes
The system SHALL support `both`, `btc-ema`, `breadth`, and `none` while preserving all ranking and execution inputs.

#### Scenario: BTC-only mode
- **WHEN** BTC is above EMA200 and breadth is below threshold
- **THEN** `btc-ema` is enabled and `both` is disabled

#### Scenario: No-filter mode
- **WHEN** both market filters are false
- **THEN** `none` is enabled

### Requirement: Sequential reproducible workflow
The ablation workflow SHALL run all four modes from existing fingerprinted candle files without fetching data.

#### Scenario: Input change
- **WHEN** a source, manifest, or candle checksum differs
- **THEN** the affected mode fails before producing a report

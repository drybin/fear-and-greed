## ADDED Requirements

### Requirement: Isolated research-v3 suite
The system SHALL preserve the core-v2 suite and permit a manifest to select the
complete research-v3 suite only through explicit preparation.

#### Scenario: Default preparation
- **WHEN** the operator omits `--suite`
- **THEN** preparation produces the unchanged core-v2 suite

#### Scenario: V3 preparation
- **WHEN** the operator supplies `--suite research-v3`
- **THEN** preparation produces exactly volatility-compression-breakout-v2 and mean-reversion-v1 with a distinct manifest identity

#### Scenario: Mixed suite
- **WHEN** a manifest combines core-v2 and research-v3 strategy codes
- **THEN** validation fails before evaluation

### Requirement: Causal v3 signals
Both v3 candidates SHALL decide from completed 1h candles and supply positive,
causal stops and 1R/2R targets to the existing next-bar execution engine.

#### Scenario: Breakout range
- **WHEN** a v2 volatility breakout is evaluated
- **THEN** the range and volume baseline exclude the breakout candle

#### Scenario: Mean reversion recovery
- **WHEN** mean reversion emits a signal
- **THEN** a prior completed RSI observation is at or below the selected oversold threshold and the current completed candle confirms recovery above RSI 35

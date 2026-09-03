## ADDED Requirements

### Requirement: Frozen pullback entry
The system SHALL freeze the entry mode and pullback parameters in the portfolio manifest and derive a distinct experiment identity.

#### Scenario: Completed trend filter
- **WHEN** a ranked candidate closes above EMA20 but more than 0.5 ATR20 above it
- **THEN** it is retained in ranking output but is not a new entry target

### Requirement: Gap-safe entry
The system SHALL reject a pullback target when the next executable open exceeds its frozen maximum entry price.

#### Scenario: Extended open
- **WHEN** a target is eligible at rebalance calculation but opens above `EMA20 + 0.5 * ATR20`
- **THEN** the allocation report records `entry_extension` and no position is opened

### Requirement: Breadth-only comparison
The candidate SHALL run with the breadth regime and unchanged portfolio accounting inputs.

#### Scenario: Baseline isolation
- **WHEN** the breadth-pullback candidate runs
- **THEN** it does not overwrite the baseline or regime-ablation reports

## ADDED Requirements

### Requirement: Point-in-time membership provenance
Every historical universe membership record SHALL retain source, effective time, observed time, rank when available, inclusion/exclusion reason, and confidence.

#### Scenario: Inferred membership
- **WHEN** membership is inferred rather than directly observed
- **THEN** the report labels it inferred and cannot classify the universe as authoritative PIT

### Requirement: Fold-time universe resolution
The system SHALL resolve training and test membership only from information available at each fold boundary.

#### Scenario: Future top-ranked asset
- **WHEN** an asset joins the top cohort after a historical fold
- **THEN** its future membership cannot make it eligible in that fold

### Requirement: Delisting-aware cohorts
Historically eligible assets SHALL remain represented after later delisting when valid membership and candle data exist.

#### Scenario: Later delisting
- **WHEN** an asset was eligible during a fold and delisted afterward
- **THEN** the later delisting does not remove its historical fold result

### Requirement: Deterministic uncertainty estimates
The system SHALL calculate manifest-seeded uncertainty intervals with explicit trade, symbol, or fold resampling units.

#### Scenario: Repeated calculation
- **WHEN** identical artifacts and seed are analyzed twice
- **THEN** intervals and diagnostics are identical

### Requirement: Multiple-testing diagnostics
Reports SHALL disclose all tested strategy versions and parameter candidates and SHALL provide configured adjusted diagnostics beside raw metrics.

#### Scenario: Attractive raw result loses adjusted support
- **WHEN** adjustment weakens evidence below the frozen threshold
- **THEN** the report preserves raw performance but downgrades the hardened evidence classification

### Requirement: Sensitivity matrix
The system SHALL compare results across declared universe, cost, fold, and neighboring-parameter perturbations.

#### Scenario: Fragile result
- **WHEN** a small declared perturbation reverses net expectancy
- **THEN** the report flags the result as fragile

### Requirement: Evidence-quality classification
The system SHALL classify evidence as `frozen-cohort`, `PIT-observational`, or `PIT-hardened` without overwriting source research artifacts.

#### Scenario: Missing reliable PIT source
- **WHEN** authoritative membership provenance is unavailable
- **THEN** the result remains `frozen-cohort` regardless of other statistics

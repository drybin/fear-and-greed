## ADDED Requirements

### Requirement: Causal slow momentum ranking
`slow-cross-sectional-momentum-v1@v1.0.0` SHALL rank each symbol by its raw
completed 60-day or 90-day daily return at a weekly Monday rebalance. The
fill-day candle SHALL NOT contribute to its own return, rank, or eligibility.
Only strictly positive-return symbols SHALL be eligible, and lexical symbol
order SHALL break equal-return ties.

#### Scenario: Fill-day isolation
- **WHEN** the current Monday candle changes after a ranking decision
- **THEN** the decision's ordering and targets remain unchanged

### Requirement: Equal-weight weekly replacement
The strategy SHALL select the top five or top ten eligible symbols and assign
them equal target weights of 20% or 10% respectively. Each weekly rebalance
SHALL close the entire preceding portfolio before opening the new target set at
the same daily open. It SHALL not use an intraweek stop.

#### Scenario: No positive momentum
- **WHEN** no scored symbol has a positive completed return
- **THEN** the strategy SHALL emit an empty target set and remain in cash

### Requirement: Development-only grid workflow
The workflow SHALL execute exactly the four frozen candidates across the five
specified pre-holdout windows. It SHALL write immutable manifests and reports,
produce a summary from those reports, and SHALL NOT open the locked holdout or
automatically select a candidate.

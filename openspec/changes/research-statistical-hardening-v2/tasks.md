## 1. Source selection and provenance

- [ ] 1.1 Define required PIT membership, rank, delisting, and source fields
- [ ] 1.2 Evaluate candidate historical universe providers for date coverage and licensing
- [ ] 1.3 Select or reject a primary source with documented evidence
- [ ] 1.4 Implement direct versus inferred provenance labels
- [ ] 1.5 Add immutable source snapshots and fingerprints

## 2. Fold-time universe

- [ ] 2.1 Implement membership resolution at each train and test boundary
- [ ] 2.2 Prevent future ranks or memberships from entering prior folds
- [ ] 2.3 Retain later-delisted assets in historically valid folds
- [ ] 2.4 Report missing candles, missing membership, and delisting exclusions separately
- [ ] 2.5 Add fixtures for joins, leaves, rank changes, delistings, and incomplete provenance

## 3. Uncertainty analysis

- [ ] 3.1 Define manifest-seeded trade-level bootstrap
- [ ] 3.2 Define symbol-clustered bootstrap
- [ ] 3.3 Define fold-clustered bootstrap
- [ ] 3.4 Add intervals for expectancy, return, profit factor, and drawdown summaries
- [ ] 3.5 Add deterministic and small-sample fixtures

## 4. Multiple testing and sensitivity

- [ ] 4.1 Count all strategy versions and parameter candidates in each research family
- [ ] 4.2 Select and document primary adjusted diagnostics
- [ ] 4.3 Report raw and adjusted evidence together
- [ ] 4.4 Implement universe, cost, fold, and parameter-neighborhood sensitivity matrix
- [ ] 4.5 Flag sign reversals and threshold failures under small perturbations
- [ ] 4.6 Add fixtures for robust and fragile synthetic results

## 5. Evidence rerun and acceptance

- [ ] 5.1 Define frozen-cohort, PIT-observational, and PIT-hardened gates
- [ ] 5.2 Run PIT eligibility comparison against existing frozen cohorts
- [ ] 5.3 Rerun selected core and portfolio experiments under new identities where data permits
- [ ] 5.4 Publish raw-versus-hardened evidence comparison
- [ ] 5.5 Preserve all source decisions and artifacts without overwrite
- [ ] 5.6 Validate and archive this change

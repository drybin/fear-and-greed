## Why

Core validation still tests current constituents backward and uses bounded but repeated hypothesis and parameter searches. Broader historical market claims and stronger confidence statements require point-in-time universes and explicit statistical controls.

## What Changes

- Add point-in-time universe membership and delisting-aware eligibility by fold date.
- Add deterministic confidence intervals for trades, folds, and strategy aggregates.
- Add multiple-testing diagnostics and false-discovery-aware reporting across strategies and parameter grids.
- Add sensitivity analysis for universe construction, costs, folds, and parameter neighborhoods.
- Reclassify evidence quality without silently rewriting earlier core or portfolio artifacts.

## Capabilities

### New Capabilities

- `statistical-research-hardening`: Point-in-time universe reconstruction, uncertainty estimation, multiple-testing controls, and evidence-quality classification.

### Modified Capabilities

None.

## Impact

- Depends on stable core report schemas from `research-validation-v2`.
- Requires an external or curated historical source for universe membership and delistings.
- Consumes existing raw candidate/fold artifacts and may trigger new experiment identities for PIT reruns.

## Dependency

Implementation MUST NOT block core validation. It begins after the core protocol and raw artifact retention are stable.

## Context

Frozen-current-cohort research is honest about its limited claim but remains survivorship-biased. Bounded grids and holdout reduce overfit but do not quantify uncertainty or account fully for repeated searches. This change upgrades evidence quality after the core workflow is operational.

## Goals / Non-Goals

**Goals:** reconstruct fold-time eligibility, retain delisted assets where data exists, quantify uncertainty, diagnose multiple testing, and publish sensitivity results.

**Non-Goals:** changing strategy behavior, execution fills, portfolio allocation, or manufacturing missing historical market data.

## Decisions

### 1. PIT source provenance is mandatory

Every membership record includes source, observed/effective timestamps, rank when available, inclusion reason, exclusion reason, and confidence. Inferred membership is labeled and cannot be presented as authoritative.

### 2. Universe is resolved at each fold boundary

Training and test eligibility use only membership and market information available at that time. Delisted or failed assets remain in historical cohorts when their candles and source records are valid.

### 3. Uncertainty is deterministic and dependency-aware

Bootstrap seeds are manifest-frozen. Resampling supports trade, symbol, and fold clusters so highly correlated trades are not automatically treated as independent.

### 4. Multiple testing is reported, not hidden

The report records the number of strategy versions and candidates considered and provides adjusted diagnostics. Raw and adjusted results remain visible; prior artifacts are not overwritten.

### 5. Hardening changes evidence labels

Results become `frozen-cohort`, `PIT-observational`, or `PIT-hardened` based on available provenance and statistical checks. A weak hardening result can downgrade confidence without deleting an earlier research decision.

## Risks / Trade-offs

- **[Historical membership may be unavailable]** → Do not infer silently; retain frozen-cohort classification.
- **[Bootstrap assumptions can mislead]** → Publish resampling unit and compare trade/symbol/fold clustered estimates.
- **[Adjustment reduces apparent significance]** → Treat that as expected evidence, not a reason to change the method.
- **[Delisted candle data may be incomplete]** → Report coverage and exclusion reasons explicitly.

## Migration Plan

1. Select and audit a PIT membership source.
2. Add provenance and fold-time universe resolution.
3. Add deterministic uncertainty and multiple-testing reports.
4. Rerun eligible experiments under new identities and publish evidence comparison.

## Open Questions

- Which provider can supply reliable historical CMC ranks, constituents, and delistings for the target period?
- Which correction should be primary: false discovery rate, family-wise error, or a conservative descriptive diagnostic only?

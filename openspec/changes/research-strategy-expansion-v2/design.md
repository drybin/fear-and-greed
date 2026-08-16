## Context

Core validation intentionally evaluates only four already-screened candidates. Those candidates are concentrated in trend pullback and breakout families. This follow-up broadens hypothesis coverage while reusing the frozen standalone execution, accounting, folds, controls, and research gates.

## Goals / Non-Goals

**Goals:**

- Add six materially distinct and falsifiable hypotheses.
- Keep every primary grid at or below 30 combinations.
- Use causal multi-timeframe indicators and next-bar execution.
- Apply the existing train-only global selection and holdout protocol unchanged.
- Decide `research-pass`, `observe`, or `reject` per version.

**Non-Goals:**

- Changing execution, sizing, core gates, or holdout semantics to improve new results.
- Shared-capital allocation or relative-strength ranking.
- Point-in-time CMC reconstruction or advanced multiple-testing correction.

## Decisions

### 1. Add hypotheses, not indicator clones

The suite adds trend continuation, trend pullback, trend-conditioned mean reversion, range mean reversion, volume-confirmed breakout, and capitulation recovery. Threshold-only alternatives remain candidates inside one strategy grid.

### 2. Every strategy is specified before data evaluation

Each version records its hypothesis, invalidation conditions, timeframes, warmup, entry confirmation, initial stop, exits, diagnostics, defaults, and complete grid before development execution.

### 3. New strategies reuse core protocol unchanged

No execution, cost, sizing, metric, fold, selection, or gate rule may be changed inside this change solely because a new strategy performs poorly. A required behavioral change creates a new core protocol change or strategy version.

### 4. Volume failure is explicit

Volume breakout and capitulation require validated volume. Missing or zero-filled volume makes a symbol ineligible; the confirmation cannot be bypassed.

### 5. Holdout remains version-specific and one-time

Development compares all six strategies under the core protocol. Only frozen versions reaching the development shortlist may open their assigned holdout once.

## Risks / Trade-offs

- **[More hypotheses increase false discoveries]** → Bound grids and retain every losing candidate; advanced correction remains a later hardening change.
- **[Market families overlap]** → Document expected regime and reject new codes that differ only by thresholds.
- **[Sparse signals]** → Use `observe` when sample gates are not met rather than loosening gates after results.
- **[Volume quality varies]** → Report strategy-specific eligibility and excluded symbols.

## Migration Plan

1. Confirm the core protocol is archived and its fixtures pass.
2. Specify and implement each strategy independently with common contract tests.
3. Freeze grids and run development across all six strategies.
4. Freeze shortlisted versions and evaluate holdout once.
5. Publish comparison against controls and the four core candidates.

## Open Questions

- Should all six strategies enter one experiment, or should sparse volume-dependent strategies use a separate manifest with the same cutoff and folds?

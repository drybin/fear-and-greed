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

Volume breakout and capitulation require validated volume. Missing, invalid, or
fully zero-filled volume makes a symbol ineligible; isolated valid zero-volume
minutes do not. The confirmation cannot be bypassed.

### 5. Holdout remains version-specific and one-time

Development compares all six strategies under the core protocol. Only frozen versions reaching the development shortlist may open their assigned holdout once.

### 6. Donchian breakout v1 is frozen as a standalone continuation hypothesis

`donchian-breakout-long-v1@v1.0.0` tests whether a 4h close crossing a
causally calculated Donchian-channel high continues an established long trend.
The last fully closed 4h candle must close above a rising EMA200; the EMA must
be higher than it was 20 completed 4h candles earlier. The channel contains
only the preceding 20 or 40 completed 4h candles, never the signal candle.

The bounded primary grid is exactly four candidates: channel length `20` or
`40`, each with an initial ATR(14) stop of `1.5` or `2.0`. A signal requires a
close above the prior channel high and prevents duplicate continuation entries
by requiring the preceding close not already to be above that same level.

The protocol engine executes at the next available 4h open. Exits are fixed:
half at 1R, the remainder at 3R, with the existing protocol breakeven move
after TP1, or a 21-day time exit. Dynamic trailing stops are explicitly out of
scope for v1 because they require a separately versioned execution-contract
change; calculating them while emitting signals would leak future candles.

This hypothesis is invalidated if development fails the unchanged sample,
profitability, stress, or robustness gates. No channel, stop, trend, or exit
threshold may be adjusted after its development evidence is observed.

### 7. EMA pullback v1 is frozen as a standalone trend-pullback hypothesis

`ema-pullback-long-v1@v1.0.0` tests whether an established higher-timeframe
uptrend can resume after a controlled pullback to a short or medium 1h EMA.
The last fully closed 4h candle must close above EMA200, and EMA200 must be
higher than 20 completed 4h candles earlier. The in-progress 4h candle is
never used for the 1h decision.

The bounded primary grid is exactly four candidates: pullback EMA `20` or
`50`, each with an initial ATR(14) distance of `1.5` or `2.0`. A completed 1h
candle must touch the selected EMA. Only a *later* completed green 1h candle
closing above that EMA confirms recovery; the touch candle cannot create an
entry by itself. The recovery opportunity expires after 12 completed hours.

The initial stop is the lower of the known touch low and entry minus the
configured ATR distance. The protocol engine executes at the next available
1h open, exits half at 1R and the remainder at 3R, and closes any remaining
position after seven days. This hypothesis is invalidated by unchanged protocol
development gates; no EMA, trend, stop, recovery, target, or timeout setting
may be retuned after development evidence is observed.

### 8. Bollinger range reversion v1 is frozen as a standalone range hypothesis

`bollinger-range-reversion-long-v1@v1.0.0` tests whether a completed 1h close
that returns inside the lower Bollinger band can revert in a non-trending
market. The preceding close must be below the lower band and ADX(14) at the
confirmation close must not exceed `20` or `25`. The band period is fixed at
20; the bounded grid contains only widths `2.0` and `2.5` standard deviations.

The initial stop is the lower of the excursion low and entry minus 1.5 ATR(14).
The protocol engine executes next-bar, takes half at the signal-time middle
band, takes the remaining position at the upper band, and closes at 48 hours
if neither exit occurs. This hypothesis is invalidated by unchanged protocol
development gates; no indicator, stop, or target threshold will be retuned
after development evidence is observed.

### 9. Capitulation reversal v1 is frozen as an event-recovery hypothesis

`capitulation-reversal-long-v1@v1.0.0` tests whether a forced 1h sell-off can
recover once a later green candle closes above the event close. An event needs
a completed close-to-close loss of at least `4%` or `6%` and volume at least
`2x` or `3x` the preceding 20-hour volume SMA. All source volumes must be
present and finite, and the file must contain at least one positive value.
Individual zero-volume minutes are allowed because Binance may validly publish
a no-trade candle; symbols with missing, invalid, or fully zero-filled volume
are excluded only for volume-dependent strategies.

Recovery is allowed for the next 12 completed hours. The initial stop is the
known event low; exits are fixed at 1R and 2R, or after 48 hours. This bounded
four-candidate grid, event definition, recovery rule, and exits cannot be
retuned after development evidence is observed.

### 10. Volume breakout v1 is frozen as a participation-confirmed hypothesis

`volume-breakout-long-v1@v1.0.0` tests whether a completed 1h break above a
causal local high is more durable when market participation is materially above
its recent baseline. The prior high is the maximum high of the preceding `20`
or `40` completed 1h candles. The signal close must exceed that high while the
preceding close does not, preventing repeated entries above an already broken
level.

The bounded primary grid is exactly four candidates: level length `20` or
`40`, each requiring current completed volume of at least `1.5x` or `2.0x` the
preceding 20-hour volume SMA. Source volume must be present, finite, and not
fully zero-filled; isolated valid no-trade minutes are permitted. A failed
volume eligibility check excludes the symbol rather than bypassing the filter.

The initial stop is fixed at 1.5 ATR(14) below the signal close. The protocol
engine executes at the next available 1h open, exits half at 1R and the
remainder at 3R, and closes any remaining position after seven days. This
hypothesis is invalidated by unchanged protocol development gates; no level,
volume, stop, target, or timeout setting may be retuned after development
evidence is observed.

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

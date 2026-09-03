## 1. Dependency and experiment scope

- [ ] 1.1 Verify `research-validation-v2` is complete and all core verification suites pass
- [ ] 1.2 Freeze the six hypothesis documents and invalidation conditions
- [ ] 1.3 Freeze strategy timeframes, warmup, defaults, stops, exits, diagnostics, and grids
- [ ] 1.4 Verify every primary grid has at most 30 unique candidates
- [ ] 1.5 Define strategy-specific data eligibility, especially volume requirements

## 2. Donchian breakout

- [x] 2.1 Implement prior-channel calculation excluding the signal candle
- [x] 2.2 Implement trend filter, breakout confirmation, ATR stop, and fixed protocol-compatible exits
- [x] 2.3 Add positive, negative, boundary, no-lookahead, and deterministic fixtures
- [x] 2.4 Add metadata, registry, serialization, and common-contract tests

## 3. EMA pullback

- [ ] 3.1 Implement causal 4h trend state and 1h EMA touch detection
- [ ] 3.2 Implement later recovery confirmation, swing-or-ATR stop, and exits
- [ ] 3.3 Add positive, negative, boundary, no-lookahead, and deterministic fixtures
- [ ] 3.4 Add metadata, registry, serialization, and common-contract tests

## 4. RSI trend mean reversion

- [x] 4.1 Implement causal 4h trend state and 1h RSI oversold state
- [x] 4.2 Implement recovery cross, ATR stop, target, and time exit
- [x] 4.3 Add positive, negative, boundary, no-lookahead, and deterministic fixtures
- [x] 4.4 Add metadata, registry, serialization, and common-contract tests

## 5. Bollinger range reversion

- [x] 5.1 Implement causal Bollinger bands and ADX range classification
- [x] 5.2 Implement excursion, re-entry, stop, and band exits
- [x] 5.3 Add positive, negative, boundary, no-lookahead, and deterministic fixtures
- [x] 5.4 Add metadata, registry, serialization, and common-contract tests

## 6. Volume breakout

- [ ] 6.1 Implement prior-level and completed-volume baseline calculations
- [ ] 6.2 Implement relative-volume confirmation, ATR stop, and exits
- [ ] 6.3 Implement explicit missing-volume ineligibility
- [ ] 6.4 Add positive, negative, missing-volume, no-lookahead, and deterministic fixtures
- [ ] 6.5 Add metadata, registry, serialization, and common-contract tests

## 7. Capitulation reversal

- [ ] 7.1 Implement trailing abnormal-return and relative-volume event detection
- [ ] 7.2 Implement later recovery, event-low stop, target, and timeout
- [ ] 7.3 Prove event detection never uses future lows
- [ ] 7.4 Add positive, negative, false-recovery, no-lookahead, and deterministic fixtures
- [ ] 7.5 Add metadata, registry, serialization, and common-contract tests

## 8. Validation and acceptance

- [ ] 8.1 Run the complete core `verify` workflow with all six strategies registered
- [ ] 8.2 Freeze a development manifest using the same controls, costs, folds, sizing, and gates as core
- [ ] 8.3 Run resumable development for all six strategies
- [ ] 8.4 Compare development results with controls and four core candidates
- [ ] 8.5 Freeze only versions passing development shortlist rules
- [ ] 8.6 Open each frozen strategy holdout once
- [ ] 8.7 Assign `research-pass`, `observe`, and `reject` without retuning
- [ ] 8.8 Publish the expanded standalone strategy report
- [ ] 8.9 Validate and archive this change before portfolio implementation

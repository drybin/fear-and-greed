## Why

After the core validation engine has produced trustworthy evidence for the four screened candidates, research should cover genuinely different market hypotheses rather than only variations of breakout logic. This change expands standalone strategy breadth without mixing in portfolio allocation or point-in-time universe work.

## What Changes

- Add six standalone spot-long hypotheses: Donchian breakout, EMA pullback, RSI trend mean reversion, Bollinger range reversion, volume-confirmed breakout, and capitulation reversal.
- Require explicit hypothesis, invalidation conditions, causal signals, initial stop, bounded grid, diagnostics, and common contract tests for each strategy.
- Run the new strategies through the already-implemented core verify/development/freeze/final protocol.
- Compare new candidates against the same controls, four core candidates, costs, folds, and research gates.

## Capabilities

### New Capabilities

- `expanded-strategy-suite`: Six orthogonal standalone strategy hypotheses and their protocol-v2 validation contracts.

### Modified Capabilities

None.

## Impact

- Depends on archived or fully implemented `research-validation-v2` capabilities.
- Adds strategy and fixture code but does not change core execution semantics.
- Does not add shared-capital portfolio behavior, relative strength, or point-in-time universes.

## Dependency

Implementation MUST NOT start until `research-validation-v2` is complete and its core research run has validated the protocol in practice.

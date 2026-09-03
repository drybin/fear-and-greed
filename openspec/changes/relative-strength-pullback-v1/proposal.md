## Why

The breadth-only regime was the best defensive configuration but the existing
90-day return divided by 30-day volatility ranking still lost money versus
cash. The current entry buys leaders at the next weekly open, which can mean
buying a stretched move rather than a continuation after a pullback.

## What Changes

- Add the separate portfolio-native `relative-strength-pullback-v1` candidate.
- Keep breadth as the sole market regime and keep the existing universe,
  ranking, costs, risk, stops, and portfolio limits frozen.
- Require a completed close above EMA20 and no more than 0.5 ATR above it
  before a ranked symbol can become a target.
- Reject a next-session open above that frozen limit as `entry_extension`.

## Non-Goals

- No retuning of lookbacks, thresholds, top-K, stops, or risk.
- No change to the completed regime ablation or its artifacts.

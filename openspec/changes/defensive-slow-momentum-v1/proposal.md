## Why

The plain slow cross-sectional momentum experiment was rejected in all four
candidate configurations. It suffered material losses in the late-2025 to
early-2026 regime change, so a follow-up must test an explicitly specified
market-protection mechanism rather than reinterpret those results.

## What Changes

- Add `defensive-slow-momentum-v1` as a portfolio-native, independently
  identified hypothesis.
- Keep the frozen raw 60-day/90-day top-5/top-10 weekly momentum grid.
- Exit all holdings at a daily open when completed prior-day data shows BTC
  below EMA-200 or positive momentum breadth below 50%; only enter on a
  healthy Monday open.
- Evaluate all four candidates on five fixed development windows without
  opening the locked holdout.

## Non-Goals

- No change to plain slow momentum, relative strength, their manifests, or
  prior reports.
- No parameter sweep of the regime thresholds, no stop-loss optimization,
  leverage, short positions, live trading, winner selection, or holdout run.

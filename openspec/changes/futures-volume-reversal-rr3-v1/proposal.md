# Futures Volume Reversal RR3 v1

## Hypothesis

A 1h sell-off of at least 4% with volume at least 2x the preceding 20-hour
average can represent capitulation. If a later completed candle within 12
hours closes green and above the sell-off close, enter long at the next hour
open. Stop is the impulse low; exit the full position at 3R or after 48 hours.

## Scope

- Frozen Binance USD-M 50-contract universe and existing hourly candles.
- Funding, commission, adverse slippage, causal next-bar entry, and stop-first
  intrabar policy are inherited from the futures execution engine.
- The single parameter point is deliberate: this experiment tests the stated
  hypothesis, rather than optimizing thresholds after observing outcomes.
- Holdout remains locked until the development review passes irreversible gates.

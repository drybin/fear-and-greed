# BTC Lead-Lag Diagnostic v1

## Why

Existing work uses BTC only as a broad market-regime filter. It does not test
whether a completed BTC impulse is followed by a delayed aligned movement in
altcoins that could support a later trade hypothesis.

## Scope

- Add a read-only diagnostic over frozen USD-M 1h candles.
- Define a BTC impulse as a completed 1h open-to-close move of at least 1% in
  either direction.
- Measure the directional altcoin return from the next synchronized 1h open
  through the close after 1, 2, 4, and 8 hours.
- Report per-symbol and aggregate means, medians, win fractions, observations,
  and the frozen-current-cohort survivorship warning.

## Out of Scope

- No order generation, position sizing, stop, take-profit, or deployment.
- No parameter sweep or holdout promotion.
- No assertion that observations are independent, as one BTC event affects many
  correlated altcoins.

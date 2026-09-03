## Hypothesis

In a positive cross-sectional breadth regime, relative-strength leaders may be
more robust when entered near their short-term trend rather than after a large
extension. The hypothesis uses EMA20 as the completed short-term trend and
ATR20 as the scale of a normal pullback.

## Frozen entry contract

At a Monday rebalance, rank the universe exactly as in the breadth-only
baseline using completed candles. A new rank-1 through rank-5 position is
eligible only when the latest completed daily close is above EMA20 and no more
than 0.5 ATR20 above EMA20. The frozen maximum entry price is
`EMA20 + 0.5 * ATR20`.

Orders fill at the next daily open only when that open does not exceed the
frozen maximum. Existing holdings retain the rank-10 exit buffer; the pullback
test applies to new entries only.

## Evaluation

Use the same source manifest, full candle checksums, base/stress cost profiles,
cash/BTC/equal-weight benchmarks, and diagnostic gates as the breadth baseline.
The candidate remains diagnostic and may only be `observe` or `reject`.

# BTC Lead-Lag RR3 Short v1

Test only the five symbols identified in the prior descriptive diagnostic:
`OPUSDT`, `INJUSDT`, `UNIUSDT`, `TIAUSDT`, and `APTUSDT`.

After a completed BTC 1h decline of at least 1%, short an eligible symbol at
the next 1h open. Stop is the high of that symbol's synchronized completed
signal candle, take-profit is 3R, and any surviving position exits after four
hours. Use the existing base futures commission and slippage model.

This is exploratory confirmation on the same data that identified the symbols;
it cannot authorize deployment or a holdout decision.

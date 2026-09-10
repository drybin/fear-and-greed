# Design

## Causal data

Each minute CSV row provides `quote_volume`, `trades`, `taker_buy_base_volume`, and `taker_buy_quote_volume`. Higher timeframes sum these fields. A 1h decision is made only at its close; the 4h trend lookup selects only a candle that had fully closed by that time.

## Entry

1. The latest completed 4h close is above a rising EMA200, measured against EMA200 twenty completed 4h bars earlier.
2. The prior 1h close is below the close three completed hours earlier.
3. The current 1h candle is green.
4. Its taker-buy quote share is at least 55% and its quote volume is at least the average of the prior twenty completed 1h candles.

The execution engine fills no earlier than the following bar. Stop is the confirmation candle low; TP1 is 1R, TP2 is 2R, and time exit is 48 hours.

## Integrity

The candidate is one frozen hypothesis, isolated as `spot-flow-pullback-v1`. It reuses the protocol's development, freeze, controls, stress, and review stages. Holdout remains explicitly locked.

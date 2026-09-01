## Experiment contract

The ablation uses the exact frozen source protocol-v2 manifest and full CSV fingerprints from the first portfolio diagnostic. Every mode has a distinct portfolio manifest identity and report path.

| Mode | Regime enabled when |
| --- | --- |
| `both` | BTC previous close is above EMA200 and positive 90-day breadth is at least 50% |
| `btc-ema` | BTC previous close is above EMA200 |
| `breadth` | positive 90-day breadth is at least 50% |
| `none` | always |

The fill-day candle remains excluded from every indicator and ranking calculation. Positions are evaluated with the same shared-capital execution engine and costs in every mode.

## Interpretation

Compare each mode against cash, BTC, and equal weight. The primary descriptive outputs are return, drawdown, average exposure, time in regime, accepted/rejected entries, trade count, and profit concentration. A mode is still `observe` or `reject` because this is a diagnostic suite; it does not authorize deployment or reopen holdout.

## Scope

`research-v3` is a separate suite selected only by `prepare --suite
research-v3`. It contains exactly two 1h, spot-long candidates and eight total
train-only parameter candidates. A manifest must contain either the complete
core-v2 suite or the complete research-v3 suite; combining them is invalid.

## Hypotheses

### Volatility Compression Breakout v2

After a low-volatility range in a rising EMA200 trend, a close above the prior
ten-bar range has better continuation odds when its completed volume is above
the prior 20-bar average. ATR compression is evaluated on the bar before the
breakout, so the breakout bar cannot satisfy its own setup.

Grid: compression factor `{0.65, 0.75}` times volume multiplier `{1.2, 1.5}`.
The stop is the prior ten-bar low; targets are 1R and 2R. Any risk wider than
15% of entry is rejected.

### Trend Mean Reversion v1

Within a rising EMA200 trend, a prior RSI(14) close at or below an oversold
threshold followed by a close above RSI 35 and above the prior close may mean
revert. The stop is the lower of the recovery and prior lows, widened only to
the configured ATR distance; targets are 1R and 2R.

Grid: oversold RSI `{25, 30}` times stop ATR `{1.2, 1.6}`. A time exit is not
included because the existing standalone execution contract has stop/target
exits only; it is explicitly deferred rather than simulated inconsistently.

## Causality

Signals are produced only from completed UTC-aligned 1h bars. Range, average
volume, ATR compression, RSI and trend values are calculated from the signal
bar and prior bars only; the execution engine fills on the next executable bar.

## Relative Strength

Relative strength is deferred. It ranks symbols and allocates scarce capital,
which requires a portfolio-level universe snapshot and shared-capital engine;
adding it here would invalidate the standalone per-symbol comparison.

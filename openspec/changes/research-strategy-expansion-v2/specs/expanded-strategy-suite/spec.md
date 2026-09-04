## ADDED Requirements

### Requirement: Expanded strategy contract
Every expanded strategy SHALL define a materially distinct hypothesis, invalidation conditions, stable version, timeframes, warmup, bounded grid, close-confirmed signal, initial stop, exits, and diagnostics.

#### Scenario: Threshold-only proposal
- **WHEN** a proposed strategy differs only by an indicator period or threshold
- **THEN** it is represented inside an existing bounded grid rather than as a new strategy code

### Requirement: Donchian breakout
`donchian-breakout-long-v1@v1.0.0` SHALL signal trend continuation only after
a completed 4h close crosses a prior-channel high calculated without the
signal candle. The close SHALL be above a rising 4h EMA200, and the preceding
close SHALL not already exceed that same prior-channel high. The bounded grid
SHALL contain only channel lengths `20` and `40` and ATR(14) stops `1.5` and
`2.0`. Exits SHALL be fixed at 1R partial, 3R final, or 21 days; dynamic
trailing stops are outside this strategy version.

#### Scenario: Causal breakout
- **WHEN** trend, prior channel, breakout close, and stop conditions pass
- **THEN** the strategy emits a next-bar long signal with channel and ATR diagnostics

### Requirement: EMA pullback
`ema-pullback-long-v1` SHALL signal recovery from a fast or medium EMA only inside a rising higher-timeframe trend.

#### Scenario: Confirmed recovery
- **WHEN** a 1h pullback touches the configured EMA and a later completed candle confirms recovery while the 4h trend passes
- **THEN** the strategy emits a next-bar signal with a swing-or-ATR stop

### Requirement: RSI trend mean reversion
`rsi-mean-reversion-long-v1` SHALL signal short-term oversold recovery only while a causal higher-timeframe long trend passes.

#### Scenario: Oversold cross back
- **WHEN** completed 1h RSI crosses above its recovery threshold after oversold state and the 4h trend passes
- **THEN** the strategy emits a signal with ATR stop, mean-reversion target, and time exit

### Requirement: Bollinger range reversion
`bollinger-range-reversion-long-v1@v1.0.0` SHALL signal lower-band re-entry
only when a completed 1h close returns inside a 20-period lower Bollinger band
after the preceding close was below it, and causal ADX(14) classifies the
market as a range. The bounded grid SHALL contain only ADX maxima `20`/`25`
and band widths `2.0`/`2.5` standard deviations. The strategy SHALL use its
fixed swing-or-1.5-ATR stop, middle/upper-band exits, and 48-hour time exit.

#### Scenario: Lower-band re-entry
- **WHEN** price closes outside the lower band and later closes back inside while ADX remains below the configured maximum
- **THEN** the strategy emits a next-bar signal with defined stop and band-based exit

### Requirement: Volume-confirmed breakout
`volume-breakout-long-v1` SHALL require a prior-level breakout and validated relative-volume confirmation.

#### Scenario: Missing volume
- **WHEN** volume is absent or fails quality validation
- **THEN** the symbol is ineligible and confirmation is not bypassed

### Requirement: Capitulation reversal
`capitulation-reversal-long-v1@v1.0.0` SHALL detect a completed 1h loss of
`4%`/`6%` with volume at least `2x`/`3x` the preceding 20-hour volume SMA, then
require a later green recovery close above the event close within 12 hours.
The strategy SHALL exclude any symbol whose source volume is absent, malformed,
non-finite, or zero. It SHALL use the known event low as stop, 1R/2R exits,
and a 48-hour time exit.

#### Scenario: Causal recovery
- **WHEN** capitulation thresholds pass and a later completed candle recovers the trigger level
- **THEN** the strategy emits a next-bar signal using the known event low as stop reference

### Requirement: Core protocol reuse
All six strategies SHALL use unchanged protocol-v2 execution, costs, sizing, controls, fold selection, holdout, and research gates.

#### Scenario: Strategy fails a gate
- **WHEN** a new strategy fails sample, cost, or robustness criteria
- **THEN** it is observed or rejected without modifying core rules in the same experiment

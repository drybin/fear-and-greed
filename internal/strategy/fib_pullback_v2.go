package strategy

import (
	"math"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	FPL2SwingPivot       = 5
	FPL2EMA200Period     = 200
	FPL2EMA20Period      = 20
	FPL2EMA200RiseBars   = 20
	FPL2MinImpulsePct    = 8.0
	FPL2MinRiskPct       = 0.004
	FPL2MaxRiskPct       = 0.04
	FPL2ExpireBars15M    = 48
	FPL2ConfirmMaxBars   = 24
	FPL2ConfirmLookback  = 3
	FPL2TP1Fraction      = 0.5
	FPL2SLBufferATRMult  = 0.1
	FPL2BOSATRMult       = 0.2
	FPL2VolSMA           = 20
	FPL2VolMult          = 1.2
	FPL2ATRPeriod        = 14
	FPL2CooldownBars     = 24
	FPL2Minutes15M       = 15
	FPL2Minutes1H        = 60
)

// FibPullbackV2Params tunes impulse threshold for sweep runs.
type FibPullbackV2Params struct {
	MinImpulsePct float64
}

func DefaultFibPullbackV2Params() FibPullbackV2Params {
	return FibPullbackV2Params{MinImpulsePct: FPL2MinImpulsePct}
}

type fpl2State int

const (
	fpl2Idle fpl2State = iota
	fpl2WaitTouch
	fpl2WaitConfirm
	fpl2InPosition
)

type fpl2Setup struct {
	impulseLow  float64
	impulseHigh float64
	swingLow    float64
	fib50       float64
	fib618      float64
	fib786      float64
	bosIdx15m   int
	expireIdx   int
	touchIdx    int
	entryPrice  float64
	entryTime   time.Time
	stopLevel   float64
	tp1Level    float64
	tp2Level    float64
	entryRisk   float64
	volumeRatio float64
	tp1Done     bool
	coins       float64
	entryContext map[string]float64
	legImpulsePct float64
	tradeEvents  []TradeEvent
}

// SimulateFibPullbackLongV2 runs filtered 1H BOS + fib pullback with structure-based TP.
func SimulateFibPullbackLongV2(candles []model.Candle) SimulationReport {
	return SimulateFibPullbackLongV2WithParams(candles, DefaultFibPullbackV2Params())
}

// SimulateFibPullbackLongV2WithParams runs v2 with configurable min impulse %.
func SimulateFibPullbackLongV2WithParams(candles []model.Candle, p FibPullbackV2Params) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	minImpulse := p.MinImpulsePct
	if minImpulse <= 0 {
		minImpulse = FPL2MinImpulsePct
	}

	min15 := FPL2EMA200Period*4 + FPL2SwingPivot*2 + FPL2ExpireBars15M + 30
	if len(candles) < min15*15 {
		return rep
	}

	c15 := AggregateMinutes(candles, FPL2Minutes15M)
	c1h := AggregateMinutes(candles, FPL2Minutes1H)
	if len(c15) < min15 || len(c1h) < FPL2EMA200Period+FPL2EMA200RiseBars+5 {
		return rep
	}

	atr := ATRWilder(c15, FPL2ATRPeriod)
	ema15 := EMA(c15, FPL2EMA200Period)
	ema20 := EMA(c15, FPL2EMA20Period)
	ema1h := EMA(c1h, FPL2EMA200Period)
	volSMA15 := smaFloats(volumeSeries(c15), FPL2VolSMA)
	volSMA1h := smaFloats(volumeSeries(c1h), FPL2VolSMA)

	h1Index := make(map[time.Time]int, len(c1h))
	for i, c := range c1h {
		h1Index[c.OpenTime] = i
	}

	state := fpl2Idle
	var setup *fpl2Setup
	var swingHighs, swingLows []fplSwingPoint
	cooldownUntil := 0
	cash := StartCash
	realizedCash := StartCash

	closeSetup := func(exitPrice float64, exitTime time.Time, exitIdx int, reason string, exitCtx map[string]float64) {
		if setup == nil || setup.coins <= 0 {
			return
		}
		cash += setup.coins * exitPrice
		setup.coins = 0
		realizedCash = cash
		if setup.entryPrice > 0 {
			rep.Trades = append(rep.Trades, Trade{
				BuyTime:      setup.entryTime,
				SellTime:     exitTime,
				WaitHours:    exitTime.Sub(setup.entryTime).Hours(),
				BuyPrice:     setup.entryPrice,
				SellPrice:    exitPrice,
				VolumeRatio:  setup.volumeRatio,
				ExitReason:   reason,
				EntryContext: CloneContext(setup.entryContext),
				ExitContext:  CloneContext(exitCtx),
				Events:       CloneEvents(setup.tradeEvents),
			})
		}
		cooldownUntil = exitIdx + FPL2CooldownBars
		state = fpl2Idle
		setup = nil
	}

	trendOK := func(hIdx, j int) bool {
		if hIdx < FPL2EMA200Period-1+FPL2EMA200RiseBars || ema1h[hIdx] <= 0 {
			return false
		}
		if c1h[hIdx].Close <= ema1h[hIdx] {
			return false
		}
		if ema1h[hIdx] <= ema1h[hIdx-FPL2EMA200RiseBars] {
			return false
		}
		if j < FPL2EMA200Period-1 || ema15[j] <= 0 {
			return false
		}
		return c15[j].Close > ema15[j]
	}

	cancelSetup := func() {
		state = fpl2Idle
		setup = nil
	}

	tryBOS := func(hIdx, j int) {
		if state != fpl2Idle || j < cooldownUntil || hIdx < FPL2SwingPivot*2 {
			return
		}
		if len(swingHighs) < 2 {
			return
		}
		legHigh := swingHighs[len(swingHighs)-1]
		prevHigh := swingHighs[len(swingHighs)-2]
		if legHigh.idx <= prevHigh.idx {
			return
		}

		var legLow fplSwingPoint
		foundLow := false
		for k := len(swingLows) - 1; k >= 0; k-- {
			if swingLows[k].idx < legHigh.idx {
				legLow = swingLows[k]
				foundLow = true
				break
			}
		}
		if !foundLow || legLow.idx >= legHigh.idx {
			return
		}

		closeH := c1h[hIdx].Close
		if hIdx == 0 || c1h[hIdx-1].Close > prevHigh.price {
			return
		}

		buf := 0.0
		if atr[j] > 0 {
			buf = atr[j] * FPL2BOSATRMult
		}
		breakLevel := prevHigh.price + buf
		if closeH <= breakLevel {
			return
		}

		if volSMA1h[hIdx] > 0 && c1h[hIdx].Volume <= volSMA1h[hIdx]*FPL2VolMult {
			return
		}

		impulsePct := (legHigh.price - legLow.price) / legLow.price * 100
		if impulsePct < minImpulse {
			return
		}
		if !trendOK(hIdx, j) {
			return
		}

		rng := legHigh.price - legLow.price
		setup = &fpl2Setup{
			impulseLow:    legLow.price,
			impulseHigh:   legHigh.price,
			swingLow:      legLow.price,
			fib50:         legHigh.price - 0.50*rng,
			fib618:        legHigh.price - 0.618*rng,
			fib786:        legHigh.price - 0.786*rng,
			bosIdx15m:     j,
			expireIdx:     j + FPL2ExpireBars15M,
			touchIdx:      -1,
			legImpulsePct: impulsePct,
		}
		state = fpl2WaitTouch
	}

	on1HClose := func(hIdx int) {
		pi := hIdx - FPL2SwingPivot
		if pi >= FPL2SwingPivot && isPivotHigh(c1h, pi, FPL2SwingPivot) {
			swingHighs = append(swingHighs, fplSwingPoint{pi, c1h[pi].High})
		}
		if pi >= FPL2SwingPivot && isPivotLow(c1h, pi, FPL2SwingPivot) {
			swingLows = append(swingLows, fplSwingPoint{pi, c1h[pi].Low})
		}
	}

	touchFibZone := func(c model.Candle) bool {
		if setup == nil {
			return false
		}
		return c.Low <= setup.fib50 && c.High >= setup.fib618
	}

	computeStop := func(j int, entry float64) float64 {
		buf := 0.0
		if atr[j] > 0 {
			buf = atr[j] * FPL2SLBufferATRMult
		}
		sl786 := setup.fib786 - buf
		slSwing := setup.swingLow - buf
		tight := math.Max(sl786, slSwing)
		wide := math.Min(sl786, slSwing)
		riskTight := entry - tight
		if riskTight < entry*FPL2MinRiskPct {
			return wide
		}
		return tight
	}

	maxHighBefore := func(j, lookback int) float64 {
		start := j - lookback
		if start < 0 {
			start = 0
		}
		var m float64
		for k := start; k < j; k++ {
			if c15[k].High > m {
				m = c15[k].High
			}
		}
		return m
	}

	for j := 0; j < len(c15); j++ {
		c := c15[j]
		bucket1h := bucketOpenUTC(c.OpenTime, FPL2Minutes1H)
		if hIdx, ok := h1Index[bucket1h]; ok {
			is1hClose := j == len(c15)-1
			if !is1hClose {
				nextBucket := bucketOpenUTC(c15[j+1].OpenTime, FPL2Minutes1H)
				is1hClose = !nextBucket.Equal(bucket1h)
			}
			if is1hClose {
				on1HClose(hIdx)
				tryBOS(hIdx, j)
			}
		}

		switch state {
		case fpl2WaitTouch, fpl2WaitConfirm:
			if setup == nil {
				state = fpl2Idle
				continue
			}
			if j > setup.expireIdx {
				cancelSetup()
				continue
			}
			if c.Close < setup.fib786 {
				cancelSetup()
				continue
			}

			if state == fpl2WaitTouch {
				if touchFibZone(c) {
					setup.touchIdx = j
					state = fpl2WaitConfirm
				}
				continue
			}

			if setup.touchIdx < 0 || j <= setup.touchIdx {
				continue
			}
			if j-setup.touchIdx > FPL2ConfirmMaxBars {
				cancelSetup()
				continue
			}
			if j <= setup.bosIdx15m {
				continue
			}
			if j < FPL2EMA20Period-1 || ema20[j] <= 0 {
				continue
			}
			refHigh := maxHighBefore(j, FPL2ConfirmLookback)
			if c.Close <= ema20[j] || c.Close <= refHigh || c.Close <= c.Open {
				continue
			}

			entry := c.Close
			stop := computeStop(j, entry)
			risk := entry - stop
			if risk <= 0 {
				cancelSetup()
				continue
			}
			if risk/entry > FPL2MaxRiskPct {
				continue
			}

			tp1 := setup.impulseHigh
			if tp1 <= entry {
				tp1 = entry + risk
			}
			tp2 := entry + 2*risk

			volRatio := 0.0
			if volSMA15[j] > 0 {
				volRatio = c.Volume / volSMA15[j]
			}
			setup.entryPrice = entry
			setup.entryTime = c.OpenTime
			setup.stopLevel = stop
			setup.tp1Level = tp1
			setup.tp2Level = tp2
			setup.entryRisk = risk
			setup.volumeRatio = volRatio
			setup.entryContext = map[string]float64{
				"fib50":        setup.fib50,
				"fib618":       setup.fib618,
				"fib786":       setup.fib786,
				"impulse_high": setup.impulseHigh,
				"impulse_low":  setup.impulseLow,
				"swing_low":    setup.swingLow,
				"ema20_15m":    ema20[j],
				"ema200_15m":   ema15[j],
				"sl":           stop,
				"tp1":          tp1,
				"tp2":          tp2,
				"risk_pct":     risk / entry * 100,
				"impulse_pct":  setup.legImpulsePct,
				"volume_ratio": volRatio,
			}
			setup.coins = cash / entry
			cash = 0
			state = fpl2InPosition

		case fpl2InPosition:
			if setup == nil {
				state = fpl2Idle
				continue
			}

			if !setup.tp1Done {
				if c.Close < setup.stopLevel {
					closeSetup(c.Close, c.OpenTime, j, ExitReasonStop, ExitCtx(c.Close, map[string]float64{
						"stop": setup.stopLevel, "tp1_done": 0,
					}))
					continue
				}
				if c.High >= setup.tp1Level {
					exitPx := setup.tp1Level
					sellCoins := setup.coins * FPL2TP1Fraction
					cash += sellCoins * exitPx
					setup.coins -= sellCoins
					setup.tp1Done = true
					realizedCash = cash
					setup.tradeEvents = append(setup.tradeEvents, TradeEvent{
						Kind: TradeEventTP1Partial, Time: c.OpenTime, Price: exitPx, Fraction: FPL2TP1Fraction,
					})
				}
				continue
			}

			if ema20[j] > 0 && c.Close < ema20[j] {
				closeSetup(c.Close, c.OpenTime, j, ExitReasonTrailEMA20, ExitCtx(c.Close, map[string]float64{
					"ema20": ema20[j], "tp1_done": 1,
				}))
				continue
			}

			if c.High >= setup.tp2Level {
				exitPx := setup.tp2Level
				if c.Close > exitPx {
					exitPx = c.Close
				}
				closeSetup(exitPx, c.OpenTime, j, ExitReasonTP2, ExitCtx(exitPx, map[string]float64{
					"tp2": setup.tp2Level, "tp1_done": 1,
				}))
			}
		}
	}

	if setup != nil && setup.coins > 0 && state == fpl2InPosition {
		rep.OpenPosition = true
		rep.RealizedCash = realizedCash
		last := c15[len(c15)-1].Close
		rep.FinalCash = cash + setup.coins*last
	} else {
		rep.RealizedCash = cash
		rep.FinalCash = cash
	}
	rep.fillStats()
	return rep
}

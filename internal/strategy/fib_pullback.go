package strategy

import (
	"math"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	FPLSwingPivot      = 5
	FPLEMA200Period    = 200
	FPLEMA20Period     = 20
	FPLEMA200RiseBars  = 20
	FPLMinImpulsePct   = 6.0
	FPLMinRiskPct      = 0.004
	FPLExpireBars15M   = 48
	FPLTP1Fraction     = 0.5
	FPLSLBufferATRMult = 0.1
	FPLVolSMA          = 20
	FPLATRPeriod       = 14
	FPLMinutes15M      = 15
	FPLMinutes1H       = 60
)

type fplState int

const (
	fplIdle fplState = iota
	fplWaitTouch
	fplWaitConfirm
	fplInPosition
)

type fplSwingPoint struct {
	idx   int
	price float64
}

type fplSetup struct {
	impulseLow  float64
	impulseHigh float64
	swingLow    float64
	fib50       float64
	fib618      float64
	fib786      float64
	bosIdx15m   int
	expireIdx   int
	entryPrice  float64
	entryTime   time.Time
	stopLevel   float64
	tp1Level    float64
	tp2Level    float64
	volumeRatio float64
	tp1Done     bool
	beActive    bool
	coins       float64
	entryContext map[string]float64
	legImpulsePct float64
	tradeEvents  []TradeEvent
}

// SimulateFibPullbackLongV1 runs 1H BOS + fib 0.5–0.618 pullback entry on 15M with 1R/2R exits.
func SimulateFibPullbackLongV1(candles []model.Candle) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	min15 := FPLEMA200Period*4 + FPLSwingPivot*2 + FPLExpireBars15M + 30
	if len(candles) < min15*15 {
		return rep
	}

	c15 := AggregateMinutes(candles, FPLMinutes15M)
	c1h := AggregateMinutes(candles, FPLMinutes1H)
	if len(c15) < min15 || len(c1h) < FPLEMA200Period+FPLEMA200RiseBars+5 {
		return rep
	}

	atr := ATRWilder(c15, FPLATRPeriod)
	ema15 := EMA(c15, FPLEMA200Period)
	ema20 := EMA(c15, FPLEMA20Period)
	ema1h := EMA(c1h, FPLEMA200Period)
	volSMA := smaFloats(volumeSeries(c15), FPLVolSMA)

	h1Index := make(map[time.Time]int, len(c1h))
	for i, c := range c1h {
		h1Index[c.OpenTime] = i
	}

	state := fplIdle
	var setup *fplSetup
	var swingHighs, swingLows []fplSwingPoint
	cash := StartCash
	realizedCash := StartCash

	closeSetup := func(exitPrice float64, exitTime time.Time, reason string, exitCtx map[string]float64) {
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
		state = fplIdle
		setup = nil
	}

	trendOK := func(hIdx, j int) bool {
		if hIdx < FPLEMA200Period-1+FPLEMA200RiseBars || ema1h[hIdx] <= 0 {
			return false
		}
		if c1h[hIdx].Close <= ema1h[hIdx] {
			return false
		}
		if ema1h[hIdx] <= ema1h[hIdx-FPLEMA200RiseBars] {
			return false
		}
		if j < FPLEMA200Period-1 || ema15[j] <= 0 {
			return false
		}
		return c15[j].Close > ema15[j]
	}

	cancelSetup := func() {
		state = fplIdle
		setup = nil
	}

	tryBOS := func(hIdx, j int) {
		if state != fplIdle || hIdx < FPLSwingPivot*2 {
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
		if hIdx == 0 || closeH <= prevHigh.price {
			return
		}
		if c1h[hIdx-1].Close > prevHigh.price {
			return
		}

		impulsePct := (legHigh.price - legLow.price) / legLow.price * 100
		if impulsePct < FPLMinImpulsePct {
			return
		}
		if !trendOK(hIdx, j) {
			return
		}

		rng := legHigh.price - legLow.price
		setup = &fplSetup{
			impulseLow:    legLow.price,
			impulseHigh:   legHigh.price,
			swingLow:      legLow.price,
			fib50:         legHigh.price - 0.50*rng,
			fib618:        legHigh.price - 0.618*rng,
			fib786:        legHigh.price - 0.786*rng,
			bosIdx15m:     j,
			expireIdx:     j + FPLExpireBars15M,
			legImpulsePct: impulsePct,
		}
		state = fplWaitTouch
	}

	on1HClose := func(hIdx int) {
		pi := hIdx - FPLSwingPivot
		if pi >= FPLSwingPivot && isPivotHigh(c1h, pi, FPLSwingPivot) {
			swingHighs = append(swingHighs, fplSwingPoint{pi, c1h[pi].High})
		}
		if pi >= FPLSwingPivot && isPivotLow(c1h, pi, FPLSwingPivot) {
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
			buf = atr[j] * FPLSLBufferATRMult
		}
		sl786 := setup.fib786 - buf
		slSwing := setup.swingLow - buf
		tight := math.Max(sl786, slSwing)
		wide := math.Min(sl786, slSwing)
		riskTight := entry - tight
		if riskTight < entry*FPLMinRiskPct {
			return wide
		}
		return tight
	}

	for j := 0; j < len(c15); j++ {
		c := c15[j]
		bucket1h := bucketOpenUTC(c.OpenTime, FPLMinutes1H)
		if hIdx, ok := h1Index[bucket1h]; ok {
			is1hClose := j == len(c15)-1
			if !is1hClose {
				nextBucket := bucketOpenUTC(c15[j+1].OpenTime, FPLMinutes1H)
				is1hClose = !nextBucket.Equal(bucket1h)
			}
			if is1hClose {
				on1HClose(hIdx)
				tryBOS(hIdx, j)
			}
		}

		switch state {
		case fplWaitTouch, fplWaitConfirm:
			if setup == nil {
				state = fplIdle
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

			if state == fplWaitTouch {
				if touchFibZone(c) {
					state = fplWaitConfirm
				}
				continue
			}

			if j <= setup.bosIdx15m {
				continue
			}
			if j < FPLEMA20Period-1 || ema20[j] <= 0 {
				continue
			}
			prevHigh := c15[j-1].High
			if c.Close <= ema20[j] || c.Close <= prevHigh {
				continue
			}

			entry := c.Close
			stop := computeStop(j, entry)
			risk := entry - stop
			if risk <= 0 {
				cancelSetup()
				continue
			}

			volRatio := 0.0
			if volSMA[j] > 0 {
				volRatio = c.Volume / volSMA[j]
			}
			setup.entryPrice = entry
			setup.entryTime = c.OpenTime
			setup.stopLevel = stop
			setup.tp1Level = entry + risk
			setup.tp2Level = entry + 2*risk
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
				"tp1":          setup.tp1Level,
				"tp2":          setup.tp2Level,
				"risk_pct":     risk / entry * 100,
				"impulse_pct":  setup.legImpulsePct,
				"volume_ratio": volRatio,
			}
			setup.coins = cash / entry
			cash = 0
			state = fplInPosition

		case fplInPosition:
			if setup == nil {
				state = fplIdle
				continue
			}

			if !setup.tp1Done {
				if c.Close < setup.stopLevel {
					closeSetup(c.Close, c.OpenTime, ExitReasonStop, ExitCtx(c.Close, map[string]float64{
						"stop": setup.stopLevel, "tp1_done": 0,
					}))
					continue
				}
				if c.High >= setup.tp1Level {
					exitPx := setup.tp1Level
					sellCoins := setup.coins * FPLTP1Fraction
					cash += sellCoins * exitPx
					setup.coins -= sellCoins
					setup.tp1Done = true
					setup.beActive = true
					realizedCash = cash
					setup.tradeEvents = append(setup.tradeEvents, TradeEvent{
						Kind: TradeEventTP1Partial, Time: c.OpenTime, Price: exitPx, Fraction: FPLTP1Fraction,
					})
				}
				continue
			}

			if setup.beActive && c.Close < setup.entryPrice {
				closeSetup(c.Close, c.OpenTime, ExitReasonBreakeven, ExitCtx(c.Close, map[string]float64{
					"entry": setup.entryPrice, "tp1_done": 1,
				}))
				continue
			}

			if c.High >= setup.tp2Level {
				exitPx := setup.tp2Level
				if c.Close > exitPx {
					exitPx = c.Close
				}
				closeSetup(exitPx, c.OpenTime, ExitReasonTP2, ExitCtx(exitPx, map[string]float64{
					"tp2": setup.tp2Level, "tp1_done": 1,
				}))
			}
		}
	}

	if setup != nil && setup.coins > 0 && state == fplInPosition {
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

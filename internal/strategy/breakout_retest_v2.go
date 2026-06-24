package strategy

import (
	"math"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	BRV2ATRPeriod       = 14
	BRV2SwingPivot      = 5
	BRV2EMAPeriod       = 200
	BRV2BreakoutATRMult = 0.2
	BRV2CancelATRMult   = 0.2
	BRV2ZoneATRMult     = 0.15
	BRV2SLATRMult       = 0.1
	BRV2VolSMA          = 20
	BRV2VolMult         = 1.3
	BRV2RetestMinBars   = 3
	BRV2RetestMaxBars   = 12
	BRV2TP1Fraction     = 0.5
	BRV2Minutes15M      = 15
	BRV2Minutes1H       = 60
)

type brv2State int

const (
	brv2Idle brv2State = iota
	brv2WaitRetest
	brv2WaitEntry
	brv2InPosition
)

type brv2Setup struct {
	swingHigh    float64
	breakoutIdx  int
	retestMaxIdx int
	retestLow    float64
	confirmIdx   int
	entryPrice   float64
	entryTime    time.Time
	stopLevel    float64
	tp1Level     float64
	tp2Level     float64
	tp1Done      bool
	beActive     bool
	coins        float64
	entryContext map[string]float64
	tradeEvents  []TradeEvent
}

// SimulateBreakoutRetestLongV2 runs breakout+retest long with EMA200 trend, volume, and R-based TP.
func SimulateBreakoutRetestLongV2(candles []model.Candle) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	minBars := BRV2EMAPeriod + BRV2SwingPivot*2 + 30
	if len(candles) < minBars*15 {
		return rep
	}

	c15 := AggregateMinutes(candles, BRV2Minutes15M)
	c1h := AggregateMinutes(candles, BRV2Minutes1H)
	if len(c15) < minBars || len(c1h) < BRV2EMAPeriod+5 {
		return rep
	}

	atr := ATRWilder(c15, BRV2ATRPeriod)
	ema15 := EMA(c15, BRV2EMAPeriod)
	ema1h := EMA(c1h, BRV2EMAPeriod)
	volSMA := smaFloats(volumeSeries(c15), BRV2VolSMA)

	h1Index := make(map[time.Time]int, len(c1h))
	for i, c := range c1h {
		h1Index[c.OpenTime] = i
	}

	state := brv2Idle
	var setup *brv2Setup
	var lastSwingHigh float64

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
				ExitReason:   reason,
				EntryContext: CloneContext(setup.entryContext),
				ExitContext:  CloneContext(exitCtx),
				Events:       CloneEvents(setup.tradeEvents),
			})
		}
		state = brv2Idle
		setup = nil
	}

	trendOK := func(j int) bool {
		if j < BRV2EMAPeriod-1 || ema15[j] <= 0 {
			return false
		}
		if c15[j].Close <= ema15[j] {
			return false
		}
		prev1h := bucketOpenUTC(c15[j].OpenTime, BRV2Minutes1H).Add(-time.Hour)
		hIdx, ok := h1Index[prev1h]
		if !ok || hIdx < BRV2EMAPeriod-1 || ema1h[hIdx] <= 0 {
			return false
		}
		return c1h[hIdx].Close > ema1h[hIdx]
	}

	touchZone := func(j int, level float64) bool {
		if setup == nil || atr[j] <= 0 {
			return false
		}
		buf := atr[j] * BRV2ZoneATRMult
		c := c15[j]
		return c.Low <= level+buf && c.High >= level-buf
	}

	for j := 0; j < len(c15); j++ {
		c := c15[j]

		if j >= BRV2SwingPivot*2 {
			pi := j - BRV2SwingPivot
			if isPivotHigh(c15, pi, BRV2SwingPivot) {
				lastSwingHigh = c15[pi].High
			}
		}

		if state == brv2Idle && j >= BRV2EMAPeriod && atr[j] > 0 && lastSwingHigh > 0 && trendOK(j) {
			volOK := volSMA[j] > 0 && c.Volume > volSMA[j]*BRV2VolMult
			breakLevel := lastSwingHigh + atr[j]*BRV2BreakoutATRMult
			if volOK && c.Close > breakLevel {
				setup = &brv2Setup{
					swingHigh:    lastSwingHigh,
					breakoutIdx:  j,
					retestMaxIdx: j + BRV2RetestMaxBars,
					retestLow:    math.MaxFloat64,
					confirmIdx:   -1,
				}
				state = brv2WaitRetest
			}
		}

		switch state {
		case brv2WaitRetest:
			if setup == nil {
				state = brv2Idle
				continue
			}
			if j <= setup.breakoutIdx {
				continue
			}

			cancelLevel := setup.swingHigh - atr[j]*BRV2CancelATRMult
			if c.Close < cancelLevel {
				state = brv2Idle
				setup = nil
				continue
			}

			barsAfter := j - setup.breakoutIdx
			if barsAfter > BRV2RetestMaxBars && setup.confirmIdx < 0 {
				state = brv2Idle
				setup = nil
				continue
			}

			if barsAfter >= BRV2RetestMinBars && barsAfter <= BRV2RetestMaxBars {
				if touchZone(j, setup.swingHigh) {
					if c.Low < setup.retestLow {
						setup.retestLow = c.Low
					}
					if c.Close > setup.swingHigh {
						setup.confirmIdx = j
						state = brv2WaitEntry
					}
				}
			}

		case brv2WaitEntry:
			if setup == nil {
				state = brv2Idle
				continue
			}
			if j <= setup.confirmIdx {
				continue
			}
			if c.Close < setup.swingHigh-atr[j]*BRV2CancelATRMult {
				state = brv2Idle
				setup = nil
				continue
			}
			entryOpen := c.Open
			if entryOpen <= 0 {
				state = brv2Idle
				setup = nil
				continue
			}
			slAtr := atr[j]
			if slAtr <= 0 {
				slAtr = atr[setup.confirmIdx]
			}
			if setup.retestLow == math.MaxFloat64 {
				setup.retestLow = c.Low
			}
			setup.stopLevel = setup.retestLow - slAtr*BRV2SLATRMult
			setup.entryPrice = entryOpen
			setup.entryTime = c.OpenTime
			risk := setup.entryPrice - setup.stopLevel
			if risk <= 0 {
				state = brv2Idle
				setup = nil
				continue
			}
			setup.tp1Level = setup.entryPrice + risk
			setup.tp2Level = setup.entryPrice + 2*risk
			setup.entryContext = map[string]float64{
				"swing_high": setup.swingHigh,
				"stop":       setup.stopLevel,
				"tp1":        setup.entryPrice + risk,
				"tp2":        setup.entryPrice + 2*risk,
				"risk_pct":   risk / setup.entryPrice * 100,
			}
			setup.coins = cash / entryOpen
			cash = 0
			state = brv2InPosition

		case brv2InPosition:
			if setup == nil {
				state = brv2Idle
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
					sellCoins := setup.coins * BRV2TP1Fraction
					cash += sellCoins * exitPx
					setup.coins -= sellCoins
					setup.tp1Done = true
					setup.beActive = true
					realizedCash = cash
					setup.tradeEvents = append(setup.tradeEvents, TradeEvent{
						Kind: TradeEventTP1Partial, Time: c.OpenTime, Price: exitPx, Fraction: BRV2TP1Fraction,
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

	if setup != nil && setup.coins > 0 && state == brv2InPosition {
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

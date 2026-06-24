package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	CRTATRPeriod     = 14
	CRTATRMult       = 1.5
	CRTVolSMA        = 20
	CRTVolMult       = 1.5
	CRTBOSLookback   = 20
	CRTExpire4HBars  = 12
	CRTSwingPivot    = 2
	CRTRRFallback    = 2.0
	CRTTP1Fraction   = 0.5
	CRTMinutes4H     = 240
	CRTMinutes15M    = 15
)

type crtState int

const (
	crtIdle crtState = iota
	crtWaitRetest
	crtWaitConfirm
	crtInPosition
)

type crtSetup struct {
	rangeHigh      float64
	rangeLow       float64
	eq             float64
	impulse4HIdx   int
	impulse15Idx   int
	sawDiscount    bool
	tp1Done        bool
	beActive       bool
	entryPrice     float64
	entryTime      time.Time
	coins          float64
	nearestSwingH  float64 // 0 = none yet
	swingFound     bool
	entryContext   map[string]float64
	tradeEvents    []TradeEvent
}

// SimulateCRTLong runs Candle Range Theory long setup (4H impulse, 15M entry/exit).
func SimulateCRTLong(candles []model.Candle) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	if len(candles) < 100 {
		return rep
	}

	c4h := AggregateMinutes(candles, CRTMinutes4H)
	c15 := AggregateMinutes(candles, CRTMinutes15M)
	if len(c4h) < CRTVolSMA+2 || len(c15) < 10 {
		return rep
	}

	atr4h := ATRWilder(c4h, CRTATRPeriod)
	volSMA4h := smaFloats(volumeSeries(c4h), CRTVolSMA)

	h4Index := make(map[time.Time]int, len(c4h))
	for i, c := range c4h {
		h4Index[c.OpenTime] = i
	}

	state := crtIdle
	var setup *crtSetup

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
		state = crtIdle
		setup = nil
	}

	tryImpulse := func(h int) {
		if state != crtIdle || h < CRTBOSLookback {
			return
		}
		if h < CRTATRPeriod-1 || h < CRTVolSMA-1 {
			return
		}
		c := c4h[h]
		if c.Close <= c.Open {
			return
		}
		rng := candleRange(c)
		if rng <= 0 || atr4h[h] <= 0 {
			return
		}
		if rng <= atr4h[h]*CRTATRMult {
			return
		}
		if volSMA4h[h] <= 0 || c.Volume <= volSMA4h[h]*CRTVolMult {
			return
		}
		prevMax := maxHighBefore(c4h, h, CRTBOSLookback)
		if c.High <= prevMax {
			return
		}
		setup = &crtSetup{
			rangeHigh:    c.High,
			rangeLow:     c.Low,
			eq:           (c.High + c.Low) / 2,
			impulse4HIdx: h,
		}
		state = crtWaitRetest
	}

	on4HClose := func(h int) {
		if setup == nil {
			return
		}
		if state == crtWaitRetest || state == crtWaitConfirm {
			if h >= setup.impulse4HIdx+CRTExpire4HBars && !setup.sawDiscount {
				state = crtIdle
				setup = nil
			}
		}
	}

	inDiscount := func(c model.Candle) bool {
		if setup == nil {
			return false
		}
		return c.Low <= setup.eq && c.High >= setup.rangeLow
	}

	bullishReaction := func(j int) bool {
		if j < 1 {
			return false
		}
		c := c15[j]
		prev := c15[j-1]
		return c.Close > c.Open && c.Close > prev.Close
	}

	updateSwingTP2 := func(j int) {
		if setup == nil || j < CRTSwingPivot*2 {
			return
		}
		i := j - CRTSwingPivot
		if i <= setup.impulse15Idx {
			return
		}
		if !isPivotHigh(c15, i, CRTSwingPivot) {
			return
		}
		h := c15[i].High
		if h <= setup.rangeHigh {
			return
		}
		if !setup.swingFound || h < setup.nearestSwingH {
			setup.nearestSwingH = h
			setup.swingFound = true
		}
	}

	tp2Target := func() float64 {
		if setup.swingFound {
			return setup.nearestSwingH
		}
		return setup.entryPrice + CRTRRFallback*(setup.entryPrice-setup.rangeLow)
	}

	for j := 0; j < len(c15); j++ {
		c := c15[j]
		bucket := bucketOpenUTC(c.OpenTime, CRTMinutes4H)
		if idx, ok := h4Index[bucket]; ok {
			is4hClose := j == len(c15)-1
			if !is4hClose {
				nextBucket := bucketOpenUTC(c15[j+1].OpenTime, CRTMinutes4H)
				is4hClose = !nextBucket.Equal(bucket)
			}
			if is4hClose {
				if state == crtIdle {
					tryImpulse(idx)
				}
				on4HClose(idx)
			}
		}

		switch state {
		case crtWaitRetest, crtWaitConfirm:
			if setup == nil {
				state = crtIdle
				continue
			}
			if c.Close < setup.rangeLow {
				state = crtIdle
				setup = nil
				continue
			}
			if inDiscount(c) {
				setup.sawDiscount = true
				state = crtWaitConfirm
			}
			if state == crtWaitConfirm && setup.sawDiscount && inDiscount(c) && bullishReaction(j) {
				if c.Close <= 0 {
					continue
				}
				setup.entryPrice = c.Close
				setup.entryTime = c.OpenTime
				setup.impulse15Idx = j
				setup.entryContext = map[string]float64{
					"range_high": setup.rangeHigh,
					"range_low":  setup.rangeLow,
					"eq":         setup.eq,
					"tp1":        setup.rangeHigh,
				}
				setup.coins = cash / c.Close
				cash = 0
				state = crtInPosition
			}

		case crtInPosition:
			if setup == nil {
				state = crtIdle
				continue
			}
			updateSwingTP2(j)

			if !setup.tp1Done {
				if c.Close < setup.rangeLow {
					closeSetup(c.Close, c.OpenTime, ExitReasonStop, ExitCtx(c.Close, map[string]float64{
						"range_low": setup.rangeLow, "tp1_done": 0,
					}))
					continue
				}
				if c.High >= setup.rangeHigh {
					exitPx := setup.rangeHigh
					sellCoins := setup.coins * CRTTP1Fraction
					cash += sellCoins * exitPx
					setup.coins -= sellCoins
					setup.tp1Done = true
					setup.beActive = true
					realizedCash = cash
					setup.tradeEvents = append(setup.tradeEvents, TradeEvent{
						Kind: TradeEventTP1Partial, Time: c.OpenTime, Price: exitPx, Fraction: CRTTP1Fraction,
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

			t2 := tp2Target()
			if c.High >= t2 {
				exitPx := t2
				if c.Close > exitPx {
					exitPx = c.Close
				}
				closeSetup(exitPx, c.OpenTime, ExitReasonTP2, ExitCtx(exitPx, map[string]float64{
					"tp2": t2, "tp1_done": 1,
				}))
			}
		}
	}

	if setup != nil && setup.coins > 0 && state == crtInPosition {
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

func volumeSeries(candles []model.Candle) []float64 {
	out := make([]float64, len(candles))
	for i, c := range candles {
		out[i] = c.Volume
	}
	return out
}

func isPivotHigh(candles []model.Candle, i, p int) bool {
	if i < p || i+p >= len(candles) {
		return false
	}
	h := candles[i].High
	for d := 1; d <= p; d++ {
		if candles[i-d].High >= h || candles[i+d].High >= h {
			return false
		}
	}
	return true
}

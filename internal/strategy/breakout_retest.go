package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	BRATRPeriod     = 14
	BRSwingPivot    = 3
	BRBufferATRMult = 0.15
	BRSLATRMult     = 0.5
	BRExpireBars    = 12
	BRTP1Fraction   = 0.5
	BRRRFallback    = 2.0
	BRMinutes15M    = 15
)

type brState int

const (
	brIdle brState = iota
	brWaitRetest
	brWaitConfirm
	brInPosition
)

type brSetup struct {
	breakoutLevel float64
	zoneTop       float64
	zoneBottom    float64
	stopLevel     float64
	impulseHigh   float64
	impulseLow    float64
	breakoutIdx   int
	expireIdx     int
	sawTouch      bool
	tp1Done       bool
	beActive      bool
	entryPrice    float64
	entryTime     time.Time
	entryIdx      int
	coins         float64
	nearestSwingH float64
	swingFound    bool
	entryContext  map[string]float64
	tradeEvents   []TradeEvent
}

// SimulateBreakoutRetestLong runs 15M swing-high breakout + retest long (CRT-style exits).
func SimulateBreakoutRetestLong(candles []model.Candle) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	if len(candles) < 200 {
		return rep
	}

	c15 := AggregateMinutes(candles, BRMinutes15M)
	if len(c15) < BRATRPeriod+BRSwingPivot*2+20 {
		return rep
	}
	atr := ATRWilder(c15, BRATRPeriod)

	state := brIdle
	var setup *brSetup
	var lastSwingHigh, lastSwingLow float64

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
		state = brIdle
		setup = nil
	}

	touchZone := func(c model.Candle) bool {
		if setup == nil {
			return false
		}
		return c.Low <= setup.zoneTop && c.High >= setup.zoneBottom && c.Close >= setup.zoneBottom
	}

	updateSwingTP2 := func(j int) {
		if setup == nil || j < BRSwingPivot*2 {
			return
		}
		i := j - BRSwingPivot
		if i <= setup.entryIdx {
			return
		}
		if !isPivotHigh(c15, i, BRSwingPivot) {
			return
		}
		h := c15[i].High
		if h <= setup.impulseHigh {
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
		risk := setup.entryPrice - setup.stopLevel
		return setup.entryPrice + BRRRFallback*risk
	}

	for j := 0; j < len(c15); j++ {
		c := c15[j]

		if j >= BRSwingPivot*2 {
			pi := j - BRSwingPivot
			if isPivotHigh(c15, pi, BRSwingPivot) {
				lastSwingHigh = c15[pi].High
			}
			if isPivotLow(c15, pi, BRSwingPivot) {
				lastSwingLow = c15[pi].Low
			}
		}

		if state == brIdle && j >= BRATRPeriod && atr[j] > 0 && lastSwingHigh > 0 {
			if c.Close > lastSwingHigh {
				buf := atr[j] * BRBufferATRMult
				impLow := lastSwingLow
				if impLow <= 0 || impLow >= c.High {
					impLow = c.Low
				}
				setup = &brSetup{
					breakoutLevel: lastSwingHigh,
					zoneTop:       lastSwingHigh + buf,
					zoneBottom:    lastSwingHigh - buf,
					stopLevel:     lastSwingHigh - atr[j]*BRSLATRMult,
					impulseHigh:   c.High,
					impulseLow:    impLow,
					breakoutIdx:   j,
					expireIdx:     j + BRExpireBars,
				}
				state = brWaitRetest
			}
		}

		switch state {
		case brWaitRetest, brWaitConfirm:
			if setup == nil {
				state = brIdle
				continue
			}
			if j > setup.breakoutIdx {
				if j > setup.expireIdx && !setup.sawTouch {
					state = brIdle
					setup = nil
					continue
				}
				if c.Close < setup.zoneBottom {
					state = brIdle
					setup = nil
					continue
				}
				if touchZone(c) {
					setup.sawTouch = true
					state = brWaitConfirm
				}
				if setup.sawTouch && c.Close >= setup.breakoutLevel && c.Close > c.Open && c.Close > 0 {
					setup.entryPrice = c.Close
					setup.entryTime = c.OpenTime
					setup.entryIdx = j
					setup.entryContext = map[string]float64{
						"breakout":     setup.breakoutLevel,
						"zone_top":     setup.zoneTop,
						"zone_bottom":  setup.zoneBottom,
						"stop":         setup.stopLevel,
						"impulse_high": setup.impulseHigh,
						"impulse_low":  setup.impulseLow,
						"tp1":          setup.impulseHigh,
					}
					setup.coins = cash / c.Close
					cash = 0
					state = brInPosition
				}
			}

		case brInPosition:
			if setup == nil {
				state = brIdle
				continue
			}
			updateSwingTP2(j)

			if !setup.tp1Done {
				if c.Close < setup.stopLevel {
					closeSetup(c.Close, c.OpenTime, ExitReasonStop, ExitCtx(c.Close, map[string]float64{
						"stop": setup.stopLevel, "tp1_done": 0,
					}))
					continue
				}
				if c.High >= setup.impulseHigh {
					exitPx := setup.impulseHigh
					sellCoins := setup.coins * BRTP1Fraction
					cash += sellCoins * exitPx
					setup.coins -= sellCoins
					setup.tp1Done = true
					setup.beActive = true
					realizedCash = cash
					setup.tradeEvents = append(setup.tradeEvents, TradeEvent{
						Kind: TradeEventTP1Partial, Time: c.OpenTime, Price: exitPx, Fraction: BRTP1Fraction,
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

	if setup != nil && setup.coins > 0 && state == brInPosition {
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

func isPivotLow(candles []model.Candle, i, p int) bool {
	if i < p || i+p >= len(candles) {
		return false
	}
	l := candles[i].Low
	for d := 1; d <= p; d++ {
		if candles[i-d].Low <= l || candles[i+d].Low <= l {
			return false
		}
	}
	return true
}

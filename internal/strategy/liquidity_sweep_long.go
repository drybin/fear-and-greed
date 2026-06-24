package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	LSLLookback      = 20
	LSLEMAPeriod     = 200
	LSLRRMultiple    = 2.0
	LSLCooldownBars  = 12
	LSLMinutes1H     = 60
)

type lslState int

const (
	lslIdle lslState = iota
	lslWaitConfirm
	lslWaitDisplacement
	lslWaitFVG
	lslWaitFVGRetest
	lslInPosition
)

type lslSetup struct {
	sweepIdx     int
	swingIdx     int
	sweepHigh    float64
	sweepLow     float64
	priorLow     float64
	poolLow1     float64
	poolLow2     float64
	entryPrice   float64
	entryTime    time.Time
	stopLevel    float64
	tpLevel      float64
	coins        float64
	fvgBottom    float64
	fvgTop       float64
	fvgIdx       int
	entryContext map[string]float64
}

// SimulateLiquiditySweepLongV1 runs 1H liquidity sweep long (lowest-low sweep + confirm bar).
func SimulateLiquiditySweepLongV1(candles []model.Candle) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	min1h := LSLEMAPeriod + LSLLookback + 5
	if len(candles) < min1h*60 {
		return rep
	}

	c1h := AggregateMinutes(candles, LSLMinutes1H)
	if len(c1h) < min1h {
		return rep
	}

	ema := EMA(c1h, LSLEMAPeriod)

	state := lslIdle
	var pending *lslSetup
	var pos *lslSetup
	cooldownUntil := 0

	cash := StartCash
	realizedCash := StartCash

	closePosition := func(exitPrice float64, exitTime time.Time, exitIdx int, reason string, exitCtx map[string]float64) {
		if pos == nil || pos.coins <= 0 {
			return
		}
		cash += pos.coins * exitPrice
		pos.coins = 0
		realizedCash = cash
		if pos.entryPrice > 0 {
			rep.Trades = append(rep.Trades, Trade{
				BuyTime:      pos.entryTime,
				SellTime:     exitTime,
				WaitHours:    exitTime.Sub(pos.entryTime).Hours(),
				BuyPrice:     pos.entryPrice,
				SellPrice:    exitPrice,
				ExitReason:   reason,
				EntryContext: CloneContext(pos.entryContext),
				ExitContext:  CloneContext(exitCtx),
			})
		}
		cooldownUntil = exitIdx + LSLCooldownBars
		state = lslIdle
		pos = nil
	}

	for j := 0; j < len(c1h); j++ {
		c := c1h[j]

		switch state {
		case lslInPosition:
			if pos == nil {
				state = lslIdle
				continue
			}
			if c.Close < pos.stopLevel {
				closePosition(c.Close, c.OpenTime, j, ExitReasonStop, ExitCtx(c.Close, map[string]float64{
					"stop": pos.stopLevel,
				}))
				continue
			}
			if c.High >= pos.tpLevel {
				exitPx := pos.tpLevel
				if c.Close > exitPx {
					exitPx = c.Close
				}
				closePosition(exitPx, c.OpenTime, j, ExitReasonTP2, ExitCtx(exitPx, map[string]float64{
					"tp2": pos.tpLevel,
				}))
			}

		case lslWaitConfirm:
			if pending == nil {
				state = lslIdle
				continue
			}
			if j <= pending.sweepIdx {
				continue
			}
			if j > pending.sweepIdx+1 {
				pending = nil
				state = lslIdle
				continue
			}
			if c.Close > pending.sweepHigh && c.Close > 0 {
				entry := c.Close
				stop := pending.sweepLow
				risk := entry - stop
				if risk <= 0 {
					pending = nil
					state = lslIdle
					continue
				}
				tp := entry + LSLRRMultiple*risk
				pos = &lslSetup{
					sweepIdx:   pending.sweepIdx,
					sweepHigh:  pending.sweepHigh,
					sweepLow:   pending.sweepLow,
					priorLow:   pending.priorLow,
					entryPrice: entry,
					entryTime:  c.OpenTime,
					stopLevel:  stop,
					tpLevel:    tp,
					entryContext: map[string]float64{
						"prior_low":  pending.priorLow,
						"sweep_low":  pending.sweepLow,
						"sweep_high": pending.sweepHigh,
						"ema200":     ema[pending.sweepIdx],
						"sl":         stop,
						"tp2":        tp,
						"risk_pct":   risk / entry * 100,
					},
				}
				pos.coins = cash / entry
				cash = 0
				pending = nil
				state = lslInPosition
			} else {
				pending = nil
				state = lslIdle
			}

		case lslIdle:
			if j < cooldownUntil {
				continue
			}
			if j < LSLEMAPeriod-1 || j < LSLLookback || ema[j] <= 0 {
				continue
			}
			if c.Close <= ema[j] {
				continue
			}
			priorLow, ok := lowestLowBefore(c1h, j, LSLLookback)
			if !ok {
				continue
			}
			if c.Low >= priorLow || c.Close <= priorLow {
				continue
			}
			pending = &lslSetup{
				sweepIdx:  j,
				sweepHigh: c.High,
				sweepLow:  c.Low,
				priorLow:  priorLow,
			}
			state = lslWaitConfirm
		}
	}

	if pos != nil && pos.coins > 0 && state == lslInPosition {
		rep.OpenPosition = true
		rep.RealizedCash = realizedCash
		last := c1h[len(c1h)-1].Close
		rep.FinalCash = cash + pos.coins*last
	} else {
		rep.RealizedCash = cash
		rep.FinalCash = cash
	}
	rep.fillStats()
	return rep
}

func lowestLowBefore(candles []model.Candle, endIdx, lookback int) (float64, bool) {
	if endIdx < lookback {
		return 0, false
	}
	start := endIdx - lookback
	minL := candles[start].Low
	for i := start + 1; i < endIdx; i++ {
		if candles[i].Low < minL {
			minL = candles[i].Low
		}
	}
	if minL <= 0 {
		return 0, false
	}
	return minL, true
}

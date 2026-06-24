package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	LSL5SwingPivot      = 2
	LSL5MaxSwingAge     = 48
	LSL5EMAPeriod       = 200
	LSL5ATRPeriod       = 14
	LSL5DispATRMult     = 1.5
	LSL5ImpulseMaxBars  = 5
	LSL5RetestMaxBars   = 12
	LSL5RRMultiple      = 2.0
	LSL5CooldownBars    = 12
	LSL5Minutes1H       = 60
)

type lsl5Swing struct {
	idx  int
	low  float64
	used bool
}

func bullishFVG(candles []model.Candle, j int) (bottom, top float64, ok bool) {
	if j < 2 {
		return 0, 0, false
	}
	bottom = candles[j-2].High
	top = candles[j].Low
	if top > bottom {
		return bottom, top, true
	}
	return 0, 0, false
}

func fvgRetestEntry(c model.Candle, fvgBottom, fvgTop float64) bool {
	if c.Low > fvgTop || c.Low < fvgBottom {
		return false
	}
	return c.Close > c.Open && c.Close > fvgBottom
}

// SimulateLiquiditySweepLongV5 runs 1H swing sweep + displacement FVG + retest entry (SMC-style).
func SimulateLiquiditySweepLongV5(candles []model.Candle) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	min1h := LSL5EMAPeriod + LSL5ATRPeriod + LSL5SwingPivot*2 + LSL5MaxSwingAge + LSL5ImpulseMaxBars + LSL5RetestMaxBars + 5
	if len(candles) < min1h*60 {
		return rep
	}

	c1h := AggregateMinutes(candles, LSL5Minutes1H)
	if len(c1h) < min1h {
		return rep
	}

	ema := EMA(c1h, LSL5EMAPeriod)
	atr := ATRWilder(c1h, LSL5ATRPeriod)

	state := lslIdle
	var pending *lslSetup
	var pos *lslSetup
	var swings []lsl5Swing
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
		cooldownUntil = exitIdx + LSL5CooldownBars
		state = lslIdle
		pos = nil
	}

	pruneSwings := func(j int) {
		keep := swings[:0]
		for _, sw := range swings {
			if j-sw.idx <= LSL5MaxSwingAge {
				keep = append(keep, sw)
			}
		}
		swings = keep
	}

	tryFVGRetestEntry := func(j int, c model.Candle) bool {
		if pending == nil {
			return false
		}
		if !fvgRetestEntry(c, pending.fvgBottom, pending.fvgTop) {
			return false
		}
		entry := c.Close
		stop := pending.sweepLow
		risk := entry - stop
		if risk <= 0 {
			return false
		}
		tp := entry + LSL5RRMultiple*risk
		pos = &lslSetup{
			sweepIdx:   pending.sweepIdx,
			swingIdx:   pending.swingIdx,
			sweepHigh:  pending.sweepHigh,
			sweepLow:   pending.sweepLow,
			priorLow:   pending.priorLow,
			fvgBottom:  pending.fvgBottom,
			fvgTop:     pending.fvgTop,
			fvgIdx:     pending.fvgIdx,
			entryPrice: entry,
			entryTime:  c.OpenTime,
			stopLevel:  stop,
			tpLevel:    tp,
			entryContext: map[string]float64{
				"swing_low":    pending.priorLow,
				"sweep_low":    pending.sweepLow,
				"sweep_high":   pending.sweepHigh,
				"ema200":       ema[pending.sweepIdx],
				"fvg_bottom":   pending.fvgBottom,
				"fvg_top":      pending.fvgTop,
				"fvg_mid":      (pending.fvgBottom + pending.fvgTop) / 2,
				"atr":          atr[j],
				"disp_atr_mult": LSL5DispATRMult,
				"sl":           stop,
				"tp2":          tp,
				"risk_pct":     risk / entry * 100,
				"pivot_bars":   LSL5SwingPivot,
			},
		}
		pos.coins = cash / entry
		cash = 0
		pending = nil
		state = lslInPosition
		return true
	}

	tryDetectFVG := func(j int) bool {
		if pending == nil || j < LSL5ATRPeriod-1 || j < 2 {
			return false
		}
		if j <= pending.sweepIdx+1 {
			return false
		}
		impulseIdx := j - 1
		if impulseIdx < pending.sweepIdx {
			return false
		}
		if !isDisplacementBar(c1h[impulseIdx], atr[impulseIdx], LSL5DispATRMult) {
			return false
		}
		bottom, top, ok := bullishFVG(c1h, j)
		if !ok {
			return false
		}
		pending.fvgBottom = bottom
		pending.fvgTop = top
		pending.fvgIdx = j
		state = lslWaitFVGRetest
		return true
	}

	for j := 0; j < len(c1h); j++ {
		c := c1h[j]

		if j >= LSL5SwingPivot*2 {
			pi := j - LSL5SwingPivot
			if isPivotLow(c1h, pi, LSL5SwingPivot) {
				swings = append(swings, lsl5Swing{idx: pi, low: c1h[pi].Low})
			}
		}
		pruneSwings(j)

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

		case lslWaitFVGRetest:
			if pending == nil {
				state = lslIdle
				continue
			}
			if j <= pending.fvgIdx {
				continue
			}
			if j > pending.fvgIdx+LSL5RetestMaxBars {
				pending = nil
				state = lslIdle
				continue
			}
			if c.Close < pending.fvgBottom || c.Close < pending.sweepLow {
				pending = nil
				state = lslIdle
				continue
			}
			if tryFVGRetestEntry(j, c) {
				continue
			}

		case lslWaitFVG:
			if pending == nil {
				state = lslIdle
				continue
			}
			if j <= pending.sweepIdx {
				continue
			}
			if j > pending.sweepIdx+LSL5ImpulseMaxBars {
				pending = nil
				state = lslIdle
				continue
			}
			tryDetectFVG(j)

		case lslIdle:
			if j < cooldownUntil {
				continue
			}
			if j < LSL5EMAPeriod-1 || ema[j] <= 0 || c.Close <= ema[j] {
				continue
			}
			for si := range swings {
				sw := &swings[si]
				if sw.used || sw.low <= 0 {
					continue
				}
				if j <= sw.idx+LSL5SwingPivot {
					continue
				}
				if c.Low >= sw.low || c.Close <= sw.low {
					continue
				}
				sw.used = true
				pending = &lslSetup{
					sweepIdx:  j,
					swingIdx:  sw.idx,
					sweepHigh: c.High,
					sweepLow:  c.Low,
					priorLow:  sw.low,
				}
				state = lslWaitFVG
				break
			}
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

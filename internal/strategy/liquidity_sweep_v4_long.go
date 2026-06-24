package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	LSL4SwingPivot     = 2
	LSL4MaxSwingAge    = 48
	LSL4EMAPeriod      = 200
	LSL4ATRPeriod      = 14
	LSL4DispATRMult    = 1.5
	LSL4DispMaxBars    = 3
	LSL4RRMultiple     = 2.0
	LSL4CooldownBars   = 12
	LSL4Minutes1H      = 60
)

type lsl4Swing struct {
	idx  int
	low  float64
	used bool
}

func isDisplacementBar(c model.Candle, atr float64, mult float64) bool {
	if atr <= 0 || c.Close <= c.Open {
		return false
	}
	return c.High-c.Low > atr*mult
}

// SimulateLiquiditySweepLongV4 runs 1H swing sweep + displacement entry (ICT-style).
func SimulateLiquiditySweepLongV4(candles []model.Candle) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	min1h := LSL4EMAPeriod + LSL4ATRPeriod + LSL4SwingPivot*2 + LSL4MaxSwingAge + 5
	if len(candles) < min1h*60 {
		return rep
	}

	c1h := AggregateMinutes(candles, LSL4Minutes1H)
	if len(c1h) < min1h {
		return rep
	}

	ema := EMA(c1h, LSL4EMAPeriod)
	atr := ATRWilder(c1h, LSL4ATRPeriod)

	state := lslIdle
	var pending *lslSetup
	var pos *lslSetup
	var swings []lsl4Swing
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
		cooldownUntil = exitIdx + LSL4CooldownBars
		state = lslIdle
		pos = nil
	}

	pruneSwings := func(j int) {
		keep := swings[:0]
		for _, sw := range swings {
			if j-sw.idx <= LSL4MaxSwingAge {
				keep = append(keep, sw)
			}
		}
		swings = keep
	}

	tryDisplacementEntry := func(j int, c model.Candle) bool {
		if pending == nil || j < LSL4ATRPeriod-1 {
			return false
		}
		if !isDisplacementBar(c, atr[j], LSL4DispATRMult) {
			return false
		}
		entry := c.Close
		stop := pending.sweepLow
		risk := entry - stop
		if risk <= 0 {
			return false
		}
		tp := entry + LSL4RRMultiple*risk
		pos = &lslSetup{
			sweepIdx:   pending.sweepIdx,
			swingIdx:   pending.swingIdx,
			sweepHigh:  pending.sweepHigh,
			sweepLow:   pending.sweepLow,
			priorLow:   pending.priorLow,
			entryPrice: entry,
			entryTime:  c.OpenTime,
			stopLevel:  stop,
			tpLevel:    tp,
			entryContext: map[string]float64{
				"swing_low":    pending.priorLow,
				"sweep_low":    pending.sweepLow,
				"sweep_high":   pending.sweepHigh,
				"ema200":       ema[pending.sweepIdx],
				"atr":          atr[j],
				"disp_range":   c.High - c.Low,
				"disp_atr_mult": LSL4DispATRMult,
				"sl":           stop,
				"tp2":          tp,
				"risk_pct":     risk / entry * 100,
				"pivot_bars":   LSL4SwingPivot,
			},
		}
		pos.coins = cash / entry
		cash = 0
		pending = nil
		state = lslInPosition
		return true
	}

	for j := 0; j < len(c1h); j++ {
		c := c1h[j]

		if j >= LSL4SwingPivot*2 {
			pi := j - LSL4SwingPivot
			if isPivotLow(c1h, pi, LSL4SwingPivot) {
				swings = append(swings, lsl4Swing{idx: pi, low: c1h[pi].Low})
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

		case lslWaitDisplacement:
			if pending == nil {
				state = lslIdle
				continue
			}
			if j <= pending.sweepIdx {
				continue
			}
			if j > pending.sweepIdx+LSL4DispMaxBars {
				pending = nil
				state = lslIdle
				continue
			}
			if tryDisplacementEntry(j, c) {
				continue
			}

		case lslIdle:
			if j < cooldownUntil {
				continue
			}
			if j < LSL4EMAPeriod-1 || ema[j] <= 0 || c.Close <= ema[j] {
				continue
			}
			for si := range swings {
				sw := &swings[si]
				if sw.used || sw.low <= 0 {
					continue
				}
				if j <= sw.idx+LSL4SwingPivot {
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
				if isDisplacementBar(c, atr[j], LSL4DispATRMult) && tryDisplacementEntry(j, c) {
					break
				}
				state = lslWaitDisplacement
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

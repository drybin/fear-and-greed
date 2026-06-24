package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	LSL2SwingPivot    = 2
	LSL2MaxSwingAge   = 48
	LSL2EMAPeriod     = 200
	LSL2RRMultiple    = 2.0
	LSL2CooldownBars  = 12
	LSL2Minutes1H     = 60
)

type lsl2Swing struct {
	idx  int
	low  float64
	used bool
}

// SimulateLiquiditySweepLongV2 runs 1H swing-pivot liquidity sweep long.
func SimulateLiquiditySweepLongV2(candles []model.Candle) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	min1h := LSL2EMAPeriod + LSL2SwingPivot*2 + LSL2MaxSwingAge + 5
	if len(candles) < min1h*60 {
		return rep
	}

	c1h := AggregateMinutes(candles, LSL2Minutes1H)
	if len(c1h) < min1h {
		return rep
	}

	ema := EMA(c1h, LSL2EMAPeriod)

	state := lslIdle
	var pending *lslSetup
	var pos *lslSetup
	var swings []lsl2Swing
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
		cooldownUntil = exitIdx + LSL2CooldownBars
		state = lslIdle
		pos = nil
	}

	pruneSwings := func(j int) {
		keep := swings[:0]
		for _, sw := range swings {
			if j-sw.idx <= LSL2MaxSwingAge {
				keep = append(keep, sw)
			}
		}
		swings = keep
	}

	for j := 0; j < len(c1h); j++ {
		c := c1h[j]

		if j >= LSL2SwingPivot*2 {
			pi := j - LSL2SwingPivot
			if isPivotLow(c1h, pi, LSL2SwingPivot) {
				swings = append(swings, lsl2Swing{idx: pi, low: c1h[pi].Low})
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
				tp := entry + LSL2RRMultiple*risk
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
						"swing_low":   pending.priorLow,
						"swing_idx":   float64(pending.swingIdx),
						"sweep_low":   pending.sweepLow,
						"sweep_high":  pending.sweepHigh,
						"ema200":      ema[pending.sweepIdx],
						"sl":          stop,
						"tp2":         tp,
						"risk_pct":    risk / entry * 100,
						"pivot_bars":  LSL2SwingPivot,
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
			if j < LSL2EMAPeriod-1 || ema[j] <= 0 || c.Close <= ema[j] {
				continue
			}
			for si := range swings {
				sw := &swings[si]
				if sw.used || sw.low <= 0 {
					continue
				}
				if j <= sw.idx+LSL2SwingPivot {
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
				state = lslWaitConfirm
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

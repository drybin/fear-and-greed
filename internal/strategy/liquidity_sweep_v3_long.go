package strategy

import (
	"math"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	LSL3SwingPivot         = 2
	LSL3EqualTolPct        = 0.2
	LSL3MaxPoolSeparation  = 24
	LSL3MaxPoolAge         = 48
	LSL3EMAPeriod          = 200
	LSL3RRMultiple         = 2.0
	LSL3CooldownBars       = 12
	LSL3Minutes1H          = 60
)

type lsl3Pivot struct {
	idx int
	low float64
}

type lsl3Pool struct {
	level  float64
	idx2   int
	low1   float64
	low2   float64
	used   bool
}

// SimulateLiquiditySweepLongV3 runs 1H equal-lows liquidity pool sweep long.
func SimulateLiquiditySweepLongV3(candles []model.Candle) SimulationReport {
	rep := SimulationReport{
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	min1h := LSL3EMAPeriod + LSL3SwingPivot*2 + LSL3MaxPoolAge + LSL3MaxPoolSeparation + 5
	if len(candles) < min1h*60 {
		return rep
	}

	c1h := AggregateMinutes(candles, LSL3Minutes1H)
	if len(c1h) < min1h {
		return rep
	}

	ema := EMA(c1h, LSL3EMAPeriod)

	state := lslIdle
	var pending *lslSetup
	var pos *lslSetup
	var pivots []lsl3Pivot
	var pools []lsl3Pool
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
		cooldownUntil = exitIdx + LSL3CooldownBars
		state = lslIdle
		pos = nil
	}

	prunePools := func(j int) {
		keep := pools[:0]
		for _, p := range pools {
			if j-p.idx2 <= LSL3MaxPoolAge {
				keep = append(keep, p)
			}
		}
		pools = keep
	}

	for j := 0; j < len(c1h); j++ {
		c := c1h[j]

		if j >= LSL3SwingPivot*2 {
			pi := j - LSL3SwingPivot
			if isPivotLow(c1h, pi, LSL3SwingPivot) {
				newLow := c1h[pi].Low
				if newLow > 0 {
					for _, p := range pivots {
						if pi-p.idx > LSL3MaxPoolSeparation {
							continue
						}
						mid := (newLow + p.low) / 2
						if mid <= 0 {
							continue
						}
						if math.Abs(newLow-p.low)/mid*100 > LSL3EqualTolPct {
							continue
						}
						level := newLow
						if p.low < level {
							level = p.low
						}
						pools = append(pools, lsl3Pool{
							level: level,
							idx2:  pi,
							low1:  p.low,
							low2:  newLow,
						})
					}
					pivots = append(pivots, lsl3Pivot{idx: pi, low: newLow})
				}
			}
		}
		prunePools(j)

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
				tp := entry + LSL3RRMultiple*risk
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
						"pool_level":  pending.priorLow,
						"equal_low1":  pending.poolLow1,
						"equal_low2":  pending.poolLow2,
						"equal_tol_pct": LSL3EqualTolPct,
						"sweep_low":   pending.sweepLow,
						"sweep_high":  pending.sweepHigh,
						"ema200":      ema[pending.sweepIdx],
						"sl":          stop,
						"tp2":         tp,
						"risk_pct":    risk / entry * 100,
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
			if j < LSL3EMAPeriod-1 || ema[j] <= 0 || c.Close <= ema[j] {
				continue
			}
			for pi := range pools {
				pool := &pools[pi]
				if pool.used || pool.level <= 0 {
					continue
				}
				if j <= pool.idx2+LSL3SwingPivot {
					continue
				}
				if c.Low >= pool.level || c.Close <= pool.level {
					continue
				}
				pool.used = true
				pending = &lslSetup{
					sweepIdx:  j,
					swingIdx:  pool.idx2,
					sweepHigh: c.High,
					sweepLow:  c.Low,
					priorLow:  pool.level,
					poolLow1:  pool.low1,
					poolLow2:  pool.low2,
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

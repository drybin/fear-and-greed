package strategy

import (
	"math/rand"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	// Rise2DayHold is minimum hold before the +2% time-based exit applies.
	Rise2DayHold = 48 * time.Hour
	// Rise2DayMinProfitPct is minimum profit % required to exit after Rise2DayHold.
	Rise2DayMinProfitPct = 2.0
)

// SimulateRandomRise2DayProfit runs random daily long entry with hybrid exits:
// before 48h — take-profit at targetPct; after 48h — exit at >= Rise2DayMinProfitPct; no stop.
func SimulateRandomRise2DayProfit(candles []model.Candle, seed int64, targetPct float64) SimulationReport {
	rep := SimulationReport{
		TargetPct: targetPct,
		StartCash: StartCash,
		FinalCash: StartCash,
	}
	if len(candles) == 0 || targetPct <= 0 {
		return rep
	}

	rng := rand.New(rand.NewSource(seed))
	cash := StartCash
	realizedCash := StartCash
	i := 0
	inPosition := false
	var buyIdx int
	var buyPrice, coins float64
	past2dBelowMin := false

	for i < len(candles) {
		if !inPosition {
			past2dBelowMin = false
			day := truncateDay(candles[i].OpenTime)
			eligible := indicesOnDayFrom(candles, day, i)
			if len(eligible) == 0 {
				i = skipToNextDay(candles, i)
				if i >= len(candles) {
					break
				}
				continue
			}
			buyIdx = eligible[rng.Intn(len(eligible))]
			buyPrice = candles[buyIdx].Close
			if buyPrice <= 0 {
				i = buyIdx + 1
				continue
			}
			coins = cash / buyPrice
			inPosition = true
			i = buyIdx + 1
			continue
		}

		c := candles[i]
		elapsed := c.OpenTime.Sub(candles[buyIdx].OpenTime)
		targetLevel := buyPrice * (1 + targetPct/100)
		minAfter2d := buyPrice * (1 + Rise2DayMinProfitPct/100)

		var (
			exit     bool
			exitPx   float64
			exitTime time.Time
			reason   string
		)

		if elapsed < Rise2DayHold {
			if c.Close >= targetLevel {
				exit = true
				exitPx = c.Close
				exitTime = c.OpenTime
				reason = ExitReasonTarget
			}
		} else {
			if c.Close < minAfter2d {
				past2dBelowMin = true
			} else {
				exit = true
				exitPx = c.Close
				exitTime = c.OpenTime
				reason = ExitReasonProfit2D
				if past2dBelowMin {
					reason = ExitReasonProfitWait
				}
			}
		}

		if exit {
			cash = coins * exitPx
			realizedCash = cash
			rep.Trades = append(rep.Trades, Trade{
				BuyTime:    candles[buyIdx].OpenTime,
				SellTime:   exitTime,
				WaitHours:  exitTime.Sub(candles[buyIdx].OpenTime).Hours(),
				BuyPrice:   buyPrice,
				SellPrice:  exitPx,
				ExitReason: reason,
				EntryContext: map[string]float64{
					"target_pct":      targetPct,
					"min_profit_pct":  Rise2DayMinProfitPct,
					"hold_hours":      Rise2DayHold.Hours(),
				},
				ExitContext: ExitCtx(exitPx, map[string]float64{
					"elapsed_hours": elapsed.Hours(),
					"target":        targetLevel,
					"min_after_2d":  minAfter2d,
				}),
			})
			inPosition = false

			nextDay, ok := nextDayWithData(candles, exitTime)
			if !ok {
				rep.RealizedCash = cash
				rep.FinalCash = cash
				rep.fillStats()
				return rep
			}
			i = firstIndexOnDay(candles, nextDay)
			continue
		}
		i++
	}

	if inPosition {
		rep.OpenPosition = true
		rep.RealizedCash = realizedCash
		rep.FinalCash = coins * candles[len(candles)-1].Close
	} else {
		rep.RealizedCash = cash
		rep.FinalCash = cash
	}
	rep.fillStats()
	return rep
}

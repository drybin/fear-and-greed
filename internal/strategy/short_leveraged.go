package strategy

import (
	"math/rand"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

// ShortLeverageParams configures isolated-margin short simulation.
type ShortLeverageParams struct {
	TargetPct float64
	Leverage  int     // 1..n
	MarginUSD float64 // margin locked per position ($)
}

// shortLiquidationPrice is the close at which isolated short loses full margin.
func shortLiquidationPrice(entry float64, leverage int) float64 {
	if leverage < 1 {
		leverage = 1
	}
	return entry * (1 + 1/float64(leverage))
}

// shortPnLUSD is P/L in USD for notional = margin * leverage.
func shortPnLUSD(margin, leverage, entry, mark float64) float64 {
	if entry <= 0 || leverage < 1 {
		return 0
	}
	notional := margin * leverage
	return notional * (entry - mark) / entry
}

// SimulateRandomShortLeveraged runs random daily short with fixed margin and leverage.
// Liquidation: close >= entry * (1 + 1/leverage) → lose margin. Cover: close <= entry * (1 - target%).
func SimulateRandomShortLeveraged(candles []model.Candle, seed int64, p ShortLeverageParams) SimulationReport {
	rep := SimulationReport{
		TargetPct: p.TargetPct,
		StartCash: StartCash,
		FinalCash: StartCash,
		Leverage:  p.Leverage,
		MarginUSD: p.MarginUSD,
	}
	if len(candles) == 0 || p.TargetPct <= 0 || p.Leverage < 1 || p.MarginUSD <= 0 {
		return rep
	}

	rng := rand.New(rand.NewSource(seed))
	cash := StartCash
	realizedCash := StartCash
	i := 0
	inPosition := false
	var entryIdx int
	var entryPrice, margin float64
	lev := p.Leverage
	liqPrice := 0.0
	tpPrice := 0.0

	for i < len(candles) {
		if rep.Bankrupt {
			break
		}

		if !inPosition {
			day := truncateDay(candles[i].OpenTime)
			eligible := indicesOnDayFrom(candles, day, i)
			if len(eligible) == 0 {
				i = skipToNextDay(candles, i)
				if i >= len(candles) {
					break
				}
				continue
			}
			entryIdx = eligible[rng.Intn(len(eligible))]
			entryPrice = candles[entryIdx].Close
			if entryPrice <= 0 {
				i = entryIdx + 1
				continue
			}
			margin = p.MarginUSD
			if margin > cash {
				margin = cash
			}
			if margin < 1 {
				i = skipToNextDay(candles, i)
				if i >= len(candles) {
					break
				}
				continue
			}
			liqPrice = shortLiquidationPrice(entryPrice, lev)
			tpPrice = entryPrice * (1 - p.TargetPct/100)
			inPosition = true
			i = entryIdx + 1
			continue
		}

		close := candles[i].Close
		if close >= liqPrice {
			cash -= margin
			rep.LiquidationCount++
			inPosition = false
			realizedCash = cash
			if cash <= 0 {
				cash = 0
				realizedCash = 0
				rep.Bankrupt = true
				break
			}
			nextDay, ok := nextDayWithData(candles, candles[i].OpenTime)
			if !ok {
				rep.RealizedCash = realizedCash
				rep.FinalCash = realizedCash
				rep.fillStats()
				return rep
			}
			i = firstIndexOnDay(candles, nextDay)
			continue
		}
		if close <= tpPrice {
			cash += shortPnLUSD(margin, float64(lev), entryPrice, close)
			realizedCash = cash
			rep.Trades = append(rep.Trades, Trade{
				BuyTime:   candles[entryIdx].OpenTime,
				SellTime:  candles[i].OpenTime,
				WaitHours: candles[i].OpenTime.Sub(candles[entryIdx].OpenTime).Hours(),
				BuyPrice:  entryPrice,
				SellPrice: close,
			})
			inPosition = false
			if cash <= 0 {
				cash = 0
				realizedCash = 0
				rep.Bankrupt = true
				break
			}
			nextDay, ok := nextDayWithData(candles, candles[i].OpenTime)
			if !ok {
				rep.RealizedCash = realizedCash
				rep.FinalCash = realizedCash
				rep.fillStats()
				return rep
			}
			i = firstIndexOnDay(candles, nextDay)
			continue
		}
		i++
	}

	if inPosition && !rep.Bankrupt {
		rep.OpenPosition = true
		rep.RealizedCash = realizedCash
		last := candles[len(candles)-1].Close
		if last >= liqPrice {
			cash -= margin
			rep.LiquidationCount++
			rep.OpenPosition = false
			if cash <= 0 {
				cash = 0
				rep.Bankrupt = true
			}
			rep.RealizedCash = cash
			rep.FinalCash = cash
		} else {
			rep.FinalCash = cash + shortPnLUSD(margin, float64(lev), entryPrice, last)
		}
	} else {
		rep.RealizedCash = cash
		rep.FinalCash = cash
	}
	rep.fillStats()
	return rep
}

// EquityPct returns total equity vs start for open short mark (used in tests).
func EquityPct(cash, margin float64, lev int, entry, mark float64) float64 {
	equity := cash + shortPnLUSD(margin, float64(lev), entry, mark)
	return (equity/StartCash - 1) * 100
}

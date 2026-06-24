package usecase

import (
	"fmt"
	"hash/fnv"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

// Fixed drop-margin position sizing (no sweep).
const DefaultDropMarginLeverage = 2
const DefaultDropMarginUSD = 30

type marginSweepBest struct {
	target    int
	leverage  int
	marginUSD int
	report    strategy.SimulationReport
}

func normalizeMarginLeverageOpts(opts *ScanMarketsOptions) {
	opts.LeverageMin = DefaultDropMarginLeverage
	opts.LeverageMax = DefaultDropMarginLeverage
	opts.MarginUSDs = []int{DefaultDropMarginUSD}
}

func hasAlgo(algos []algoKind, a algoKind) bool {
	for _, x := range algos {
		if x == a {
			return true
		}
	}
	return false
}

func runMarginLeverageSweep(
	candles []model.Candle,
	symbol, period string,
	opts ScanMarketsOptions,
) *marginSweepBest {
	var best *marginSweepBest
	for lev := opts.LeverageMin; lev <= opts.LeverageMax; lev++ {
		for _, margin := range opts.MarginUSDs {
			for target := opts.TargetMin; target <= opts.TargetMax; target += opts.TargetStep {
				seed := seedForMargin(symbol, period, target, lev, margin, opts.Seed)
				rep := strategy.SimulateRandomShortLeveraged(candles, seed, strategy.ShortLeverageParams{
					TargetPct: float64(target),
					Leverage:  lev,
					MarginUSD: float64(margin),
				})
				if best == nil || rep.ProfitPct > best.report.ProfitPct {
					best = &marginSweepBest{
						target:    target,
						leverage:  lev,
						marginUSD: margin,
						report:    rep,
					}
				}
			}
		}
	}
	return best
}

func marginScore(symbol string, period periodKind, b *marginSweepBest) runScore {
	return runScore{
		symbol:       symbol,
		period:       period,
		algo:         algoShortMarginSweep,
		sellTarget:   b.target,
		leverage:     b.leverage,
		marginUSD:    b.marginUSD,
		profitPct:    b.report.ProfitPct,
		profitUSD:    b.report.ProfitUSD,
		tradeCount:   b.report.CompletedCount,
		liquidations: b.report.LiquidationCount,
		bankrupt:     b.report.Bankrupt,
	}
}

func seedForMargin(symbol, period string, targetPct, leverage, marginUSD int, base int64) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(symbol))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(period))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte("drop-margin"))
	_, _ = h.Write([]byte{0})
	_, _ = fmt.Fprintf(h, "%d:%d:%d", targetPct, leverage, marginUSD)
	return int64(h.Sum64()) ^ base
}

func printMarginSweepBest(symbol, periodTitle string, candles []model.Candle, best *marginSweepBest, minTrades int) {
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")
	rep := best.report

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoShortMarginSweep))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Println("    (isolated short: margin locked per trade; liq when close >= entry×(1+1/leverage))")
	fmt.Printf("    >> best: target %d%% | leverage %dx | margin $%d\n",
		best.target, best.leverage, best.marginUSD)
	fmt.Printf("       profit %+.2f%% ($%.2f) | trades %d | liquidations %d",
		rep.ProfitPct, rep.ProfitUSD, rep.CompletedCount, rep.LiquidationCount)
	if rep.Bankrupt {
		fmt.Print(" | BANKRUPT")
	}
	fmt.Print(formatOpenLegNote(rep))
	fmt.Println()
	if rep.CompletedCount < minTrades {
		fmt.Printf("    (warning: best has <%d trades; try lower target or shorter period)\n", minTrades)
	}
	fmt.Println()
}

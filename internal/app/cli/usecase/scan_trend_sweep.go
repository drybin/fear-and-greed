package usecase

import (
	"fmt"
	"hash/fnv"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

type trendSweepBest struct {
	longTarget  int
	shortTarget int
	report      strategy.SimulationReport
}

const trendLongTargetMax = 30
const trendShortTargetMax = 30

func runTrendSweep(
	candles []model.Candle,
	symbol, period string,
	opts ScanMarketsOptions,
) *trendSweepBest {
	cache := strategy.NewTrendDailyCache(candles, []int{strategy.TrendSMAPeriod})

	var best *trendSweepBest
	longMax := opts.TargetMax
	if longMax > trendLongTargetMax {
		longMax = trendLongTargetMax
	}
	shortMax := opts.TargetMax
	if shortMax > trendShortTargetMax {
		shortMax = trendShortTargetMax
	}
	for longT := opts.TargetMin; longT <= longMax; longT += opts.TargetStep {
		for shortT := opts.TargetMin; shortT <= shortMax; shortT += opts.TargetStep {
			seed := seedForTrend(symbol, period, longT, shortT, opts.Seed)
			rep := strategy.SimulateTrendAdaptiveWithCache(candles, seed, strategy.TrendParams{
				LongTargetPct:  float64(longT),
				ShortTargetPct: float64(shortT),
			}, cache)
			if best == nil || rep.ProfitPct > best.report.ProfitPct {
				best = &trendSweepBest{longTarget: longT, shortTarget: shortT, report: rep}
			}
		}
	}
	return best
}

func trendScore(symbol string, period periodKind, b *trendSweepBest) runScore {
	return runScore{
		symbol:       symbol,
		period:       period,
		algo:         algoTrend,
		sellTarget:   b.longTarget,
		shortTarget:  b.shortTarget,
		profitPct:    b.report.ProfitPct,
		profitUSD:    b.report.ProfitUSD,
		tradeCount:   b.report.CompletedCount,
		liquidations: b.report.LiquidationCount,
		bankrupt:     b.report.Bankrupt,
	}
}

func seedForTrend(symbol, period string, longT, shortT int, base int64) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(symbol))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(period))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte("trend"))
	_, _ = h.Write([]byte{0})
	_, _ = fmt.Fprintf(h, "%d:%d", longT, shortT)
	return int64(h.Sum64()) ^ base
}

func printTrendSweepBest(symbol, periodTitle string, candles []model.Candle, best *trendSweepBest, minTrades int) {
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")
	rep := best.report

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoTrend))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    trend: daily SMA(%d), prev day close > SMA → long, < SMA → short $%d 1x (liq on)\n",
		strategy.TrendSMAPeriod, strategy.TrendMarginUSD)
	fmt.Printf("    >> best: long +%d%% | short -%d%%\n", best.longTarget, best.shortTarget)
	fmt.Printf("       profit %+.2f%% ($%.2f) | trades %d | liquidations %d",
		rep.ProfitPct, rep.ProfitUSD, rep.CompletedCount, rep.LiquidationCount)
	if rep.Bankrupt {
		fmt.Print(" | BANKRUPT")
	}
	fmt.Print(formatOpenLegNote(rep))
	fmt.Println()
	if rep.CompletedCount < minTrades {
		fmt.Printf("    (warning: best has <%d trades)\n", minTrades)
	}
	fmt.Println()
}

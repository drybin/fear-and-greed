package usecase

import (
	"fmt"
	"hash/fnv"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

type trendLongSweepBest struct {
	longTarget int
	report     strategy.SimulationReport
}

func runTrendLongSweep(
	candles []model.Candle,
	symbol, period string,
	opts ScanMarketsOptions,
) *trendLongSweepBest {
	cache := strategy.NewTrendDailyCache(candles, []int{strategy.TrendSMAPeriod})

	var best *trendLongSweepBest
	for longT := opts.TargetMin; longT <= opts.TargetMax; longT += opts.TargetStep {
		seed := seedForTrendLong(symbol, period, longT, opts.Seed)
		rep := strategy.SimulateTrendLongOnlySMAWithCache(
			candles,
			seed,
			float64(longT),
			strategy.TrendSMAPeriod,
			cache,
		)
		if best == nil || rep.ProfitPct > best.report.ProfitPct {
			best = &trendLongSweepBest{longTarget: longT, report: rep}
		}
	}
	return best
}

func trendLongScore(symbol string, period periodKind, b *trendLongSweepBest) runScore {
	return runScore{
		symbol:       symbol,
		period:       period,
		algo:         algoTrendLong,
		sellTarget:   b.longTarget,
		profitPct:    b.report.ProfitPct,
		profitUSD:    b.report.ProfitUSD,
		tradeCount:   b.report.CompletedCount,
		liquidations: b.report.LiquidationCount,
		bankrupt:     b.report.Bankrupt,
	}
}

func seedForTrendLong(symbol, period string, longT int, base int64) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(symbol))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(period))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte("trend-long"))
	_, _ = h.Write([]byte{0})
	_, _ = fmt.Fprintf(h, "%d", longT)
	return int64(h.Sum64()) ^ base
}

func printTrendLongSweepBest(symbol, periodTitle string, candles []model.Candle, best *trendLongSweepBest, minTrades int) {
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")
	rep := best.report

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoTrendLong))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    trend: daily SMA(%d), prev day close > SMA → long only\n", strategy.TrendSMAPeriod)
	fmt.Printf("    >> best: long +%d%%\n", best.longTarget)
	fmt.Printf("       profit %+.2f%% ($%.2f) | trades %d",
		rep.ProfitPct, rep.ProfitUSD, rep.CompletedCount)
	fmt.Print(formatOpenLegNote(rep))
	fmt.Println()
	if rep.CompletedCount < minTrades {
		fmt.Printf("    (warning: best has <%d trades)\n", minTrades)
	}
	fmt.Println()
}

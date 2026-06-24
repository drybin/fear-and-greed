package usecase

import (
	"fmt"
	"hash/fnv"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

type trendLongSMARetestSweepBest struct {
	smaPeriod  int
	longTarget int
	report     strategy.SimulationReport
}

type trendLongSMARetestSweepResult struct {
	perSMA []smaPerRow
	best   *trendLongSMARetestSweepBest
}

func runTrendLongSMARetestSweep(
	candles []model.Candle,
	symbol, period string,
	opts ScanMarketsOptions,
) *trendLongSMARetestSweepResult {
	var smaPeriods []int
	for sma := trendLongSMAMin; sma <= trendLongSMAMax; sma += trendLongSMAStep {
		smaPeriods = append(smaPeriods, sma)
	}
	cache := strategy.NewTrendDailyCache(candles, smaPeriods)

	res := &trendLongSMARetestSweepResult{perSMA: make([]smaPerRow, 0, len(smaPeriods))}
	for sma := trendLongSMAMin; sma <= trendLongSMAMax; sma += trendLongSMAStep {
		var bestForSMA *smaPerRow
		for longT := opts.TargetMin; longT <= opts.TargetMax; longT += opts.TargetStep {
			seed := seedForTrendLongSMARetest(symbol, period, sma, longT, opts.Seed)
			rep := strategy.SimulateTrendLongRetestSMAWithCache(
				candles,
				seed,
				float64(longT),
				sma,
				opts.RetestEpsilonPct,
				opts.RetestLookaheadCandles,
				cache,
			)
			row := smaPerRow{smaPeriod: sma, longTarget: longT, report: rep}
			if bestForSMA == nil || rep.ProfitPct > bestForSMA.report.ProfitPct {
				bestForSMA = &row
			}
			if res.best == nil || rep.ProfitPct > res.best.report.ProfitPct {
				res.best = &trendLongSMARetestSweepBest{smaPeriod: sma, longTarget: longT, report: rep}
			}
		}
		if bestForSMA != nil {
			res.perSMA = append(res.perSMA, *bestForSMA)
		}
	}
	if res.best == nil {
		return nil
	}
	return res
}

func trendLongSMARetestScore(symbol string, period periodKind, b *trendLongSMARetestSweepBest) runScore {
	return runScore{
		symbol:       symbol,
		period:       period,
		algo:         algoTrendLongSMARetest,
		sellTarget:   b.longTarget,
		smaPeriod:    b.smaPeriod,
		profitPct:    b.report.ProfitPct,
		profitUSD:    b.report.ProfitUSD,
		tradeCount:   b.report.CompletedCount,
		liquidations: b.report.LiquidationCount,
		bankrupt:     b.report.Bankrupt,
	}
}

func seedForTrendLongSMARetest(symbol, period string, sma, longT int, base int64) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(symbol))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(period))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte("trend-long-sma-retest"))
	_, _ = h.Write([]byte{0})
	_, _ = fmt.Fprintf(h, "%d:%d", sma, longT)
	return int64(h.Sum64()) ^ base
}

func printTrendLongSMARetestSweepResult(
	symbol, periodTitle string,
	candles []model.Candle,
	res *trendLongSMARetestSweepResult,
	minTrades int,
	smaReport string,
	epsilonPct float64,
	lookahead int,
) {
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoTrendLongSMARetest))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    trend: close breakout above SMA + retest (eps %.3f%%, <=%d candles), SMA sweep %d..%d step %d\n",
		epsilonPct, lookahead, trendLongSMAMin, trendLongSMAMax, trendLongSMAStep)

	if smaReport == smaReportAll && len(res.perSMA) > 0 {
		fmt.Println("    best target per SMA:")
		printSMAPerRowsTable(res.perSMA)
	}

	rep := res.best.report
	fmt.Printf("    >> best: SMA(%d) | long +%d%%\n", res.best.smaPeriod, res.best.longTarget)
	fmt.Printf("       profit %+.2f%% ($%.2f) | trades %d",
		rep.ProfitPct, rep.ProfitUSD, rep.CompletedCount)
	fmt.Print(formatOpenLegNote(rep))
	fmt.Println()
	if rep.CompletedCount < minTrades {
		fmt.Printf("    (warning: best has <%d trades)\n", minTrades)
	}
	fmt.Println()
}

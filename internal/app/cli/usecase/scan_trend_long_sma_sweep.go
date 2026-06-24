package usecase

import (
	"fmt"
	"hash/fnv"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

const (
	trendLongSMAMin  = 10
	trendLongSMAMax  = 100
	trendLongSMAStep = 10
)

type trendLongSMASweepBest struct {
	smaPeriod  int
	longTarget int
	report     strategy.SimulationReport
}

type trendLongSMASweepResult struct {
	perSMA []smaPerRow
	best   *trendLongSMASweepBest
}

func runTrendLongSMASweep(
	candles []model.Candle,
	symbol, period string,
	opts ScanMarketsOptions,
) *trendLongSMASweepResult {
	var smaPeriods []int
	for sma := trendLongSMAMin; sma <= trendLongSMAMax; sma += trendLongSMAStep {
		smaPeriods = append(smaPeriods, sma)
	}
	cache := strategy.NewTrendDailyCache(candles, smaPeriods)

	targets := make([]float64, 0, 1+(opts.TargetMax-opts.TargetMin)/opts.TargetStep)
	targetInts := make([]int, 0, cap(targets))
	for longT := opts.TargetMin; longT <= opts.TargetMax; longT += opts.TargetStep {
		targets = append(targets, float64(longT))
		targetInts = append(targetInts, longT)
	}

	res := &trendLongSMASweepResult{perSMA: make([]smaPerRow, 0, len(smaPeriods))}
	for sma := trendLongSMAMin; sma <= trendLongSMAMax; sma += trendLongSMAStep {
		seeds := make([]int64, len(targetInts))
		for i, longT := range targetInts {
			seeds[i] = seedForTrendLongSMA(symbol, period, sma, longT, opts.Seed)
		}
		reps := strategy.SimulateTrendLongOnlySMASweepWithCache(candles, sma, targets, seeds, cache)

		var bestForSMA *smaPerRow
		for i, rep := range reps {
			row := smaPerRow{smaPeriod: sma, longTarget: targetInts[i], report: rep}
			if bestForSMA == nil || rep.ProfitPct > bestForSMA.report.ProfitPct {
				bestForSMA = &row
			}
			if res.best == nil || rep.ProfitPct > res.best.report.ProfitPct {
				res.best = &trendLongSMASweepBest{smaPeriod: sma, longTarget: targetInts[i], report: rep}
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

func trendLongSMAScore(symbol string, period periodKind, b *trendLongSMASweepBest) runScore {
	return runScore{
		symbol:       symbol,
		period:       period,
		algo:         algoTrendLongSMA,
		sellTarget:   b.longTarget,
		smaPeriod:    b.smaPeriod,
		profitPct:    b.report.ProfitPct,
		profitUSD:    b.report.ProfitUSD,
		tradeCount:   b.report.CompletedCount,
		liquidations: b.report.LiquidationCount,
		bankrupt:     b.report.Bankrupt,
	}
}

func seedForTrendLongSMA(symbol, period string, sma, longT int, base int64) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(symbol))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(period))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte("trend-long-sma"))
	_, _ = h.Write([]byte{0})
	_, _ = fmt.Fprintf(h, "%d:%d", sma, longT)
	return int64(h.Sum64()) ^ base
}

func printTrendLongSMASweepResult(symbol, periodTitle string, candles []model.Candle, res *trendLongSMASweepResult, minTrades int, smaReport string) {
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoTrendLongSMA))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    trend: daily SMA sweep %d..%d step %d | prev day close > SMA → long only\n",
		trendLongSMAMin, trendLongSMAMax, trendLongSMAStep)

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

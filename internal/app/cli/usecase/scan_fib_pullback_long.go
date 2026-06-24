package usecase

import (
	"fmt"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

func runFibPullbackLong(candles []model.Candle) strategy.SimulationReport {
	return strategy.SimulateFibPullbackLongV1(candles)
}

func printFibPullbackLongBest(symbol, periodTitle string, candles []model.Candle, rep strategy.SimulationReport, minTrades int) {
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoFibPullbackLong))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    fib pullback v1: 1H BOS swing N=%d | fib 0.5-0.618 | EMA200 1H+15M rising | entry EMA20+prevHigh | TP 1R/2R\n",
		strategy.FPLSwingPivot)
	fmt.Printf("       profit %+.2f%% ($%.2f) | trades %d",
		rep.ProfitPct, rep.ProfitUSD, rep.CompletedCount)
	fmt.Print(formatOpenLegNote(rep))
	fmt.Println()
	if rep.CompletedCount > 0 {
		var sumVol float64
		var n int
		for _, tr := range rep.Trades {
			if tr.VolumeRatio > 0 {
				sumVol += tr.VolumeRatio
				n++
			}
		}
		if n > 0 {
			fmt.Printf("       avg entry volumeRatio: %.2f\n", sumVol/float64(n))
		}
	}
	if rep.CompletedCount < minTrades && !rep.OpenPosition {
		fmt.Printf("    (warning: <%d completed trades)\n", minTrades)
	}
	fmt.Println()
}

func fibPullbackLongScore(symbol string, period periodKind, rep strategy.SimulationReport) runScore {
	return runScore{
		symbol:     symbol,
		period:     period,
		algo:       algoFibPullbackLong,
		profitPct:  rep.ProfitPct,
		profitUSD:  rep.ProfitUSD,
		tradeCount: rep.CompletedCount,
	}
}

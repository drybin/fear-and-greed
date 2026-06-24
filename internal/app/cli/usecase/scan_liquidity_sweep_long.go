package usecase

import (
	"fmt"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

func runLiquiditySweepLong(candles []model.Candle) strategy.SimulationReport {
	return strategy.SimulateLiquiditySweepLongV1(candles)
}

func printLiquiditySweepLongBest(symbol, periodTitle string, candles []model.Candle, rep strategy.SimulationReport, minTrades int) {
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoLiquiditySweepLong))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    1H sweep: low<%d-bar low + close>prior | EMA200 | confirm close>sweep high | SL sweep low | TP %.0fR\n",
		strategy.LSLLookback, strategy.LSLRRMultiple)
	fmt.Printf("       profit %+.2f%% ($%.2f) | trades %d",
		rep.ProfitPct, rep.ProfitUSD, rep.CompletedCount)
	fmt.Print(formatOpenLegNote(rep))
	fmt.Println()
	if rep.CompletedCount < minTrades && !rep.OpenPosition {
		fmt.Printf("    (warning: <%d completed trades)\n", minTrades)
	}
	fmt.Println()
}

func liquiditySweepLongScore(symbol string, period periodKind, rep strategy.SimulationReport) runScore {
	return runScore{
		symbol:     symbol,
		period:     period,
		algo:       algoLiquiditySweepLong,
		profitPct:  rep.ProfitPct,
		profitUSD:  rep.ProfitUSD,
		tradeCount: rep.CompletedCount,
	}
}

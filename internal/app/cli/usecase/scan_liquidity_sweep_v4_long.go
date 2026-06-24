package usecase

import (
	"fmt"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

func runLiquiditySweepLongV4(candles []model.Candle) strategy.SimulationReport {
	return strategy.SimulateLiquiditySweepLongV4(candles)
}

func printLiquiditySweepLongV4Best(symbol, periodTitle string, candles []model.Candle, rep strategy.SimulationReport, minTrades int) {
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoLiquiditySweepLongV4))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    1H swing sweep + displacement (bullish, range>ATR14×%.1f) within %d bars | EMA200 | SL sweep low | TP %.0fR\n",
		strategy.LSL4DispATRMult, strategy.LSL4DispMaxBars, strategy.LSL4RRMultiple)
	fmt.Printf("       profit %+.2f%% ($%.2f) | trades %d",
		rep.ProfitPct, rep.ProfitUSD, rep.CompletedCount)
	fmt.Print(formatOpenLegNote(rep))
	fmt.Println()
	if rep.CompletedCount < minTrades && !rep.OpenPosition {
		fmt.Printf("    (warning: <%d completed trades)\n", minTrades)
	}
	fmt.Println()
}

func liquiditySweepLongV4Score(symbol string, period periodKind, rep strategy.SimulationReport) runScore {
	return runScore{
		symbol:     symbol,
		period:     period,
		algo:       algoLiquiditySweepLongV4,
		profitPct:  rep.ProfitPct,
		profitUSD:  rep.ProfitUSD,
		tradeCount: rep.CompletedCount,
	}
}

package usecase

import (
	"fmt"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

func runBreakoutRetestLongV2(candles []model.Candle) strategy.SimulationReport {
	return strategy.SimulateBreakoutRetestLongV2(candles)
}

func printBreakoutRetestLongV2Best(symbol, periodTitle string, candles []model.Candle, rep strategy.SimulationReport, minTrades int) {
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoBreakoutRetestLongV2))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    BR v2: EMA200 1H+15M | swing N=%d | break+ATR×%.1f vol×%.1f | retest %d-%d bars | entry next open | TP 1R/2R\n",
		strategy.BRV2SwingPivot, strategy.BRV2BreakoutATRMult, strategy.BRV2VolMult,
		strategy.BRV2RetestMinBars, strategy.BRV2RetestMaxBars)
	fmt.Printf("       profit %+.2f%% ($%.2f) | trades %d",
		rep.ProfitPct, rep.ProfitUSD, rep.CompletedCount)
	fmt.Print(formatOpenLegNote(rep))
	fmt.Println()
	if rep.CompletedCount < minTrades && !rep.OpenPosition {
		fmt.Printf("    (warning: <%d completed trades)\n", minTrades)
	}
	fmt.Println()
}

func breakoutRetestLongV2Score(symbol string, period periodKind, rep strategy.SimulationReport) runScore {
	return runScore{
		symbol:     symbol,
		period:     period,
		algo:       algoBreakoutRetestLongV2,
		profitPct:  rep.ProfitPct,
		profitUSD:  rep.ProfitUSD,
		tradeCount: rep.CompletedCount,
	}
}

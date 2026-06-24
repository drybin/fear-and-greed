package usecase

import (
	"fmt"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

type volatilityCompressionBreakoutV1SweepResult struct {
	best volatilityCompressionBreakoutV1SweepRow
	rows []volatilityCompressionBreakoutV1SweepRow
}

type volatilityCompressionBreakoutV1SweepRow struct {
	compWindow  int
	rangeWindow int
	atrComp     float64
	expansion   float64
	trendFilter strategy.NR7TrendFilter
	report      strategy.SimulationReport
}

func runVolatilityCompressionBreakoutV1Sweep(candles []model.Candle) *volatilityCompressionBreakoutV1SweepResult {
	var rows []volatilityCompressionBreakoutV1SweepRow
	for _, cw := range strategy.VolatilityCompressionBreakoutV1SweepCompressionWindows() {
		for _, rw := range strategy.VolatilityCompressionBreakoutV1SweepRangeWindows() {
			for _, ac := range strategy.VolatilityCompressionBreakoutV1SweepATRCompression() {
				for _, exp := range strategy.VolatilityCompressionBreakoutV1SweepBreakoutExpansion() {
					for _, tf := range strategy.NR7TrendBreakoutV1SweepTrendFilters() {
						rep := strategy.SimulateVolatilityCompressionBreakoutV1WithParams(candles, strategy.VolatilityCompressionBreakoutV1Params{
							CompressionWindow: cw,
							RangeWindow:       rw,
							ATRCompression:    ac,
							BreakoutExpansion: exp,
							TrendFilter:       tf,
							StopMode:          strategy.VCBStopCompressionLow,
						})
						rows = append(rows, volatilityCompressionBreakoutV1SweepRow{
							compWindow:  cw,
							rangeWindow: rw,
							atrComp:     ac,
							expansion:   exp,
							trendFilter: tf,
							report:      rep,
						})
					}
				}
			}
		}
	}
	res := &volatilityCompressionBreakoutV1SweepResult{rows: rows}
	res.best = pickBestVolatilityCompressionBreakoutV1Row(rows)
	return res
}

func pickBestVolatilityCompressionBreakoutV1Row(rows []volatilityCompressionBreakoutV1SweepRow) volatilityCompressionBreakoutV1SweepRow {
	if len(rows) == 0 {
		return volatilityCompressionBreakoutV1SweepRow{}
	}
	var qualified *volatilityCompressionBreakoutV1SweepRow
	for i := range rows {
		r := &rows[i]
		if r.report.CompletedCount == 0 && !r.report.OpenPosition {
			continue
		}
		if qualified == nil || r.report.ProfitPct > qualified.report.ProfitPct {
			qualified = r
		}
	}
	if qualified != nil {
		return *qualified
	}
	best := &rows[0]
	for i := range rows {
		r := &rows[i]
		if r.report.CompletedCount > best.report.CompletedCount ||
			(r.report.CompletedCount == best.report.CompletedCount && r.report.ProfitPct > best.report.ProfitPct) {
			best = r
		}
	}
	return *best
}

func printVolatilityCompressionBreakoutV1Sweep(symbol, periodTitle string, candles []model.Candle, res *volatilityCompressionBreakoutV1SweepResult, minTrades int) {
	if res == nil {
		return
	}
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoVolatilityCompressionBreakoutV1))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    VCB v1: 1H ATR compression + range breakout | SL compressionLow | TP 1R/2R\n")
	fmt.Printf("    sweep: compWin %v | rangeWin %v | ATR× %v | expansion %v | trend ×3\n",
		strategy.VolatilityCompressionBreakoutV1SweepCompressionWindows(),
		strategy.VolatilityCompressionBreakoutV1SweepRangeWindows(),
		strategy.VolatilityCompressionBreakoutV1SweepATRCompression(),
		strategy.VolatilityCompressionBreakoutV1SweepBreakoutExpansion())
	fmt.Printf("    grid size: %d combinations\n", len(res.rows))
	b := res.best
	if b.report.CompletedCount == 0 && !b.report.OpenPosition {
		fmt.Printf("    >> best: n/a (0 trades on all combinations)\n")
	} else {
		fmt.Printf("    >> best: comp%d range%d ATR×%.1f exp%.1f %s | profit %+.2f%% ($%.2f, %d trades)%s\n",
			b.compWindow, b.rangeWindow, b.atrComp, b.expansion, b.trendFilter.Label(),
			b.report.ProfitPct, b.report.ProfitUSD, b.report.CompletedCount,
			formatOpenLegNote(b.report))
	}
	if b.report.CompletedCount < minTrades && !b.report.OpenPosition {
		fmt.Printf("    (warning: <%d completed trades on best combo)\n", minTrades)
	}
	fmt.Println()
}

func volatilityCompressionBreakoutV1Score(symbol string, period periodKind, res *volatilityCompressionBreakoutV1SweepResult) runScore {
	if res == nil {
		return runScore{symbol: symbol, period: period, algo: algoVolatilityCompressionBreakoutV1}
	}
	return runScore{
		symbol:     symbol,
		period:     period,
		algo:       algoVolatilityCompressionBreakoutV1,
		sellTarget: res.best.compWindow,
		smaPeriod:  res.best.rangeWindow,
		profitPct:  res.best.report.ProfitPct,
		profitUSD:  res.best.report.ProfitUSD,
		tradeCount: res.best.report.CompletedCount,
	}
}

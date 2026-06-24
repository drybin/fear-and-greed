package usecase

import (
	"fmt"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

var (
	nr7LengthSweep      = []int{5, 7, 10}
	nr7CompressionSweep = []float64{0.6, 0.8, 1.0}
	nr7LifetimeSweep    = []int{6, 12, 24}
)

type nr7TrendBreakoutV1SweepResult struct {
	best nr7TrendBreakoutV1SweepRow
	rows []nr7TrendBreakoutV1SweepRow
}

type nr7TrendBreakoutV1SweepRow struct {
	nrLength     int
	compression  float64
	lifetime     int
	trendFilter  strategy.NR7TrendFilter
	report       strategy.SimulationReport
}

func runNR7TrendBreakoutV1Sweep(candles []model.Candle) *nr7TrendBreakoutV1SweepResult {
	var rows []nr7TrendBreakoutV1SweepRow
	for _, n := range nr7LengthSweep {
		for _, comp := range nr7CompressionSweep {
			for _, life := range nr7LifetimeSweep {
				for _, tf := range strategy.NR7TrendBreakoutV1SweepTrendFilters() {
					rep := strategy.SimulateNR7TrendBreakoutV1WithParams(candles, strategy.NR7TrendBreakoutV1Params{
						NRLength:       n,
						ATRCompression: comp,
						SetupLifetime:  life,
						TrendFilter:    tf,
					})
					rows = append(rows, nr7TrendBreakoutV1SweepRow{
						nrLength:    n,
						compression: comp,
						lifetime:    life,
						trendFilter: tf,
						report:      rep,
					})
				}
			}
		}
	}
	res := &nr7TrendBreakoutV1SweepResult{rows: rows}
	res.best = pickBestNR7TrendBreakoutV1Row(rows)
	return res
}

func pickBestNR7TrendBreakoutV1Row(rows []nr7TrendBreakoutV1SweepRow) nr7TrendBreakoutV1SweepRow {
	if len(rows) == 0 {
		return nr7TrendBreakoutV1SweepRow{}
	}
	var qualified *nr7TrendBreakoutV1SweepRow
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

func printNR7TrendBreakoutV1Sweep(symbol, periodTitle string, candles []model.Candle, res *nr7TrendBreakoutV1SweepResult, minTrades int) {
	if res == nil {
		return
	}
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoNR7TrendBreakoutV1))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    NR trend breakout v1: 1H NR compression + vol breakout | SL nrLow | TP 1R/2R\n")
	fmt.Printf("    sweep: NR len %v | ATR× %v | lifetime %v | trend filters ×3\n",
		nr7LengthSweep, nr7CompressionSweep, nr7LifetimeSweep)
	fmt.Printf("    grid size: %d combinations\n", len(res.rows))
	b := res.best
	if b.report.CompletedCount == 0 && !b.report.OpenPosition {
		fmt.Printf("    >> best: n/a (0 trades on all combinations)\n")
	} else {
		fmt.Printf("    >> best: NR%d ATR×%.1f life%d %s | profit %+.2f%% ($%.2f, %d trades)%s\n",
			b.nrLength, b.compression, b.lifetime, b.trendFilter.Label(),
			b.report.ProfitPct, b.report.ProfitUSD, b.report.CompletedCount,
			formatOpenLegNote(b.report))
	}
	if b.report.CompletedCount < minTrades && !b.report.OpenPosition {
		fmt.Printf("    (warning: <%d completed trades on best combo)\n", minTrades)
	}
	fmt.Println()
}

func nr7TrendBreakoutV1Score(symbol string, period periodKind, res *nr7TrendBreakoutV1SweepResult) runScore {
	if res == nil {
		return runScore{symbol: symbol, period: period, algo: algoNR7TrendBreakoutV1}
	}
	return runScore{
		symbol:     symbol,
		period:     period,
		algo:       algoNR7TrendBreakoutV1,
		sellTarget: res.best.nrLength,
		smaPeriod:  res.best.lifetime,
		profitPct:  res.best.report.ProfitPct,
		profitUSD:  res.best.report.ProfitUSD,
		tradeCount: res.best.report.CompletedCount,
	}
}

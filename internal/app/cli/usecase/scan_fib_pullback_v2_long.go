package usecase

import (
	"fmt"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

var fibPullbackV2ImpulseSweep = []float64{6, 8, 10}

type fibPullbackV2SweepResult struct {
	best     fibPullbackV2SweepRow
	rows     []fibPullbackV2SweepRow
}

type fibPullbackV2SweepRow struct {
	minImpulse int
	report     strategy.SimulationReport
}

func runFibPullbackLongV2Sweep(candles []model.Candle) *fibPullbackV2SweepResult {
	var rows []fibPullbackV2SweepRow
	for _, imp := range fibPullbackV2ImpulseSweep {
		rep := strategy.SimulateFibPullbackLongV2WithParams(candles, strategy.FibPullbackV2Params{
			MinImpulsePct: imp,
		})
		rows = append(rows, fibPullbackV2SweepRow{
			minImpulse: int(imp),
			report:     rep,
		})
	}
	res := &fibPullbackV2SweepResult{rows: rows}
	res.best = pickBestFibV2Row(rows)
	return res
}

func pickBestFibV2Row(rows []fibPullbackV2SweepRow) fibPullbackV2SweepRow {
	if len(rows) == 0 {
		return fibPullbackV2SweepRow{}
	}
	var qualified *fibPullbackV2SweepRow
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

func printFibPullbackLongV2Sweep(symbol, periodTitle string, candles []model.Candle, res *fibPullbackV2SweepResult, minTrades int) {
	if res == nil {
		return
	}
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoFibPullbackLongV2))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    fib pullback v2: BOS+ATR×%.1f vol×%.1f | entry confirm %d bars | TP1 impulseHigh trail EMA20 | maxRisk %.1f%% cooldown %d\n",
		strategy.FPL2BOSATRMult, strategy.FPL2VolMult, strategy.FPL2ConfirmLookback,
		strategy.FPL2MaxRiskPct*100, strategy.FPL2CooldownBars)
	fmt.Println("    (profit %/$ = closed trades only; open leg at end shown separately if any)")
	fmt.Printf("    %8s  %7s  %10s  %10s\n", "impulse%", "trades", "profit %", "profit $")
	for _, row := range res.rows {
		fmt.Printf("    %7d%%  %7d  %10s  %10.2f\n",
			row.minImpulse,
			row.report.CompletedCount,
			formatProfitPct(row.report),
			formatProfitUSD(row.report),
		)
	}
	if res.best.report.CompletedCount == 0 && !res.best.report.OpenPosition {
		fmt.Printf("    >> best impulse: n/a (0 trades on all sweep levels)\n")
	} else {
		b := res.best
		fmt.Printf("    >> best impulse: %d%%  profit %+.2f%% ($%.2f, %d trades)%s\n",
			b.minImpulse, b.report.ProfitPct, b.report.ProfitUSD, b.report.CompletedCount,
			formatOpenLegNote(b.report))
	}
	if res.best.report.CompletedCount > 0 {
		var sumVol float64
		var n int
		for _, tr := range res.best.report.Trades {
			if tr.VolumeRatio > 0 {
				sumVol += tr.VolumeRatio
				n++
			}
		}
		if n > 0 {
			fmt.Printf("       avg entry volumeRatio (best): %.2f\n", sumVol/float64(n))
		}
	}
	if res.best.report.CompletedCount < minTrades && !res.best.report.OpenPosition {
		fmt.Printf("    (warning: <%d completed trades on best impulse)\n", minTrades)
	}
	fmt.Println()
}

func fibPullbackLongV2Score(symbol string, period periodKind, res *fibPullbackV2SweepResult) runScore {
	if res == nil {
		return runScore{symbol: symbol, period: period, algo: algoFibPullbackLongV2}
	}
	return runScore{
		symbol:     symbol,
		period:     period,
		algo:       algoFibPullbackLongV2,
		sellTarget: res.best.minImpulse,
		profitPct:  res.best.report.ProfitPct,
		profitUSD:  res.best.report.ProfitUSD,
		tradeCount: res.best.report.CompletedCount,
	}
}

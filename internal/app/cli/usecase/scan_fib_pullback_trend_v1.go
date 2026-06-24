package usecase

import (
	"fmt"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

var (
	fibTrendV1ImpulseSweep = []float64{5, 8, 10, 15}
	fibTrendV1PivotSweep   = []int{3, 5, 7}
	fibTrendV1WaitSweep    = []int{24, 48, 96}
)

type fibPullbackTrendV1SweepResult struct {
	best fibPullbackTrendV1SweepRow
	rows []fibPullbackTrendV1SweepRow
}

type fibPullbackTrendV1SweepRow struct {
	minImpulse int
	pivot      int
	zone       strategy.FibPullbackTrendZone
	maxWait    int
	report     strategy.SimulationReport
}

func runFibPullbackTrendV1Sweep(candles []model.Candle) *fibPullbackTrendV1SweepResult {
	var rows []fibPullbackTrendV1SweepRow
	for _, imp := range fibTrendV1ImpulseSweep {
		for _, pivot := range fibTrendV1PivotSweep {
			for _, zone := range strategy.FibPullbackTrendV1SweepZones() {
				for _, wait := range fibTrendV1WaitSweep {
					rep := strategy.SimulateFibPullbackTrendV1WithParams(candles, strategy.FibPullbackTrendV1Params{
						PivotLength:    pivot,
						MinImpulsePct:  imp,
						Zone:           zone,
						MaxWaitBars15M: wait,
					})
					rows = append(rows, fibPullbackTrendV1SweepRow{
						minImpulse: int(imp),
						pivot:      pivot,
						zone:       zone,
						maxWait:    wait,
						report:     rep,
					})
				}
			}
		}
	}
	res := &fibPullbackTrendV1SweepResult{rows: rows}
	res.best = pickBestFibTrendV1Row(rows)
	return res
}

func pickBestFibTrendV1Row(rows []fibPullbackTrendV1SweepRow) fibPullbackTrendV1SweepRow {
	if len(rows) == 0 {
		return fibPullbackTrendV1SweepRow{}
	}
	var qualified *fibPullbackTrendV1SweepRow
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

func printFibPullbackTrendV1Sweep(symbol, periodTitle string, candles []model.Candle, res *fibPullbackTrendV1SweepResult, minTrades int) {
	if res == nil {
		return
	}
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoSweepTitle(algoFibPullbackTrendV1))
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Printf("    fib pullback trend v1 (spec): 1H BOS last swing high | 15M fib retest | SL fib786 | TP 1R/2R\n")
	fmt.Printf("    sweep: impulse%% %v | pivot %v | zone 0.382-0.618/0.5-0.618/0.5-0.786 | maxWait %v\n",
		fibTrendV1ImpulseSweep, fibTrendV1PivotSweep, fibTrendV1WaitSweep)
	fmt.Printf("    grid size: %d combinations\n", len(res.rows))
	b := res.best
	if b.report.CompletedCount == 0 && !b.report.OpenPosition {
		fmt.Printf("    >> best: n/a (0 trades on all combinations)\n")
	} else {
		fmt.Printf("    >> best: impulse %d%% pivot %d zone %s wait %d | profit %+.2f%% ($%.2f, %d trades)%s\n",
			b.minImpulse, b.pivot, b.zone.Label(), b.maxWait,
			b.report.ProfitPct, b.report.ProfitUSD, b.report.CompletedCount,
			formatOpenLegNote(b.report))
	}
	if b.report.CompletedCount < minTrades && !b.report.OpenPosition {
		fmt.Printf("    (warning: <%d completed trades on best combo)\n", minTrades)
	}
	fmt.Println()
}

func fibPullbackTrendV1Score(symbol string, period periodKind, res *fibPullbackTrendV1SweepResult) runScore {
	if res == nil {
		return runScore{symbol: symbol, period: period, algo: algoFibPullbackTrendV1}
	}
	return runScore{
		symbol:     symbol,
		period:     period,
		algo:       algoFibPullbackTrendV1,
		sellTarget: res.best.minImpulse,
		smaPeriod:  res.best.pivot,
		profitPct:  res.best.report.ProfitPct,
		profitUSD:  res.best.report.ProfitUSD,
		tradeCount: res.best.report.CompletedCount,
	}
}

package usecase

import (
	"fmt"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/infrastructure/scanreport"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

func reportCandleMeta(candles []model.Candle) scanreport.CandleRange {
	if len(candles) == 0 {
		return scanreport.CandleRange{}
	}
	return scanreport.CandleRange{
		From:  candles[0].OpenTime,
		To:    candles[len(candles)-1].OpenTime,
		Count: len(candles),
	}
}

func reportBase(algo string, symbol string, p periodSpec, candles []model.Candle) scanreport.Result {
	meta := reportCandleMeta(candles)
	return scanreport.Result{
		Algo:        algo,
		Symbol:      symbol,
		Period:      string(p.kind),
		PeriodLabel: p.title,
		CandleFrom:  meta.From.UTC().Format(time.RFC3339),
		CandleTo:    meta.To.UTC().Format(time.RFC3339),
		CandleCount: meta.Count,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
}

func bestFromReport(rep strategy.SimulationReport, paramLabel string, paramValue int) scanreport.Best {
	return scanreport.Best{
		ParamLabel:   paramLabel,
		ParamValue:   paramValue,
		ProfitPct:    rep.ProfitPct,
		ProfitUSD:    rep.ProfitUSD,
		TradeCount:   rep.CompletedCount,
		OpenPosition: rep.OpenPosition,
		WaitHoursAvg: rep.WaitHoursAvg,
		Liquidations: rep.LiquidationCount,
		Bankrupt:     rep.Bankrupt,
		Leverage:     rep.Leverage,
		MarginUSD:    int(rep.MarginUSD),
	}
}

func sweepRowFromReport(paramLabel string, paramValue int, rep strategy.SimulationReport) scanreport.SweepRow {
	return scanreport.SweepRow{
		ParamLabel: paramLabel,
		ParamValue: paramValue,
		ProfitPct:  rep.ProfitPct,
		ProfitUSD:  rep.ProfitUSD,
		TradeCount: rep.CompletedCount,
	}
}

func attachTrades(r *scanreport.Result, rep strategy.SimulationReport) {
	r.Trades = scanreport.TradesFromStrategy(rep.Trades)
}

func persistReport(w *scanreport.Writer, r scanreport.Result, candles []model.Candle) {
	if w == nil {
		return
	}
	_ = w.Save(r)
	_ = scanreport.SaveOHLC(w.Root(), r, candles, scanreport.DefaultChartIntervalMin)
}

func (u *ScanMarkets) saveSimReport(w *scanreport.Writer, algo algoKind, symbol string, p periodSpec, candles []model.Candle, rep strategy.SimulationReport, paramLabel string, paramValue int) {
	if w == nil {
		return
	}
	r := reportBase(string(algo), symbol, p, candles)
	r.Best = bestFromReport(rep, paramLabel, paramValue)
	attachTrades(&r, rep)
	persistReport(w, r, candles)
}

func (u *ScanMarkets) saveSweepReport(w *scanreport.Writer, algo algoKind, symbol string, p periodSpec, candles []model.Candle, rows []sweepRow, best *sweepRow) {
	if w == nil || best == nil {
		return
	}
	r := reportBase(string(algo), symbol, p, candles)
	for _, row := range rows {
		r.Sweep = append(r.Sweep, sweepRowFromReport(
			fmt.Sprintf("target %d%%", row.target), row.target, row.report))
	}
	r.Best = bestFromReport(best.report, fmt.Sprintf("target %d%%", best.target), best.target)
	attachTrades(&r, best.report)
	persistReport(w, r, candles)
}

func (u *ScanMarkets) saveTrendReport(w *scanreport.Writer, algo algoKind, symbol string, p periodSpec, candles []model.Candle, best *trendSweepBest) {
	if w == nil || best == nil {
		return
	}
	r := reportBase(string(algo), symbol, p, candles)
	label := fmt.Sprintf("long +%d%% short -%d%%", best.longTarget, best.shortTarget)
	r.Best = bestFromReport(best.report, label, best.longTarget)
	r.Best.LongTarget = best.longTarget
	r.Best.ShortTarget = best.shortTarget
	attachTrades(&r, best.report)
	persistReport(w, r, candles)
}

func (u *ScanMarkets) saveTrendLongReport(w *scanreport.Writer, algo algoKind, symbol string, p periodSpec, candles []model.Candle, best *trendLongSweepBest) {
	if w == nil || best == nil {
		return
	}
	r := reportBase(string(algo), symbol, p, candles)
	r.Best = bestFromReport(best.report, fmt.Sprintf("long +%d%%", best.longTarget), best.longTarget)
	r.Best.LongTarget = best.longTarget
	attachTrades(&r, best.report)
	persistReport(w, r, candles)
}

func (u *ScanMarkets) saveTrendLongSMAReport(w *scanreport.Writer, algo algoKind, symbol string, p periodSpec, candles []model.Candle, res *trendLongSMASweepResult) {
	if w == nil || res == nil || res.best == nil {
		return
	}
	r := reportBase(string(algo), symbol, p, candles)
	for _, row := range res.perSMA {
		r.Sweep = append(r.Sweep, scanreport.SweepRow{
			ParamLabel: fmt.Sprintf("SMA(%d) +%d%%", row.smaPeriod, row.longTarget),
			ParamValue: row.longTarget,
			ProfitPct:  row.report.ProfitPct,
			ProfitUSD:  row.report.ProfitUSD,
			TradeCount: row.report.CompletedCount,
			SMAPeriod:  row.smaPeriod,
			LongTarget: row.longTarget,
		})
	}
	b := res.best
	r.Best = bestFromReport(b.report, fmt.Sprintf("SMA(%d) +%d%%", b.smaPeriod, b.longTarget), b.longTarget)
	r.Best.SMAPeriod = b.smaPeriod
	r.Best.LongTarget = b.longTarget
	attachTrades(&r, b.report)
	persistReport(w, r, candles)
}

func (u *ScanMarkets) saveTrendLongSMARetestReport(w *scanreport.Writer, algo algoKind, symbol string, p periodSpec, candles []model.Candle, res *trendLongSMARetestSweepResult) {
	if w == nil || res == nil || res.best == nil {
		return
	}
	r := reportBase(string(algo), symbol, p, candles)
	for _, row := range res.perSMA {
		r.Sweep = append(r.Sweep, scanreport.SweepRow{
			ParamLabel: fmt.Sprintf("SMA(%d) +%d%%", row.smaPeriod, row.longTarget),
			ParamValue: row.longTarget,
			ProfitPct:  row.report.ProfitPct,
			ProfitUSD:  row.report.ProfitUSD,
			TradeCount: row.report.CompletedCount,
			SMAPeriod:  row.smaPeriod,
			LongTarget: row.longTarget,
		})
	}
	b := res.best
	r.Best = bestFromReport(b.report, fmt.Sprintf("SMA(%d) +%d%%", b.smaPeriod, b.longTarget), b.longTarget)
	r.Best.SMAPeriod = b.smaPeriod
	r.Best.LongTarget = b.longTarget
	attachTrades(&r, b.report)
	persistReport(w, r, candles)
}

func (u *ScanMarkets) saveMarginReport(w *scanreport.Writer, algo algoKind, symbol string, p periodSpec, candles []model.Candle, best *marginSweepBest) {
	if w == nil || best == nil {
		return
	}
	r := reportBase(string(algo), symbol, p, candles)
	label := fmt.Sprintf("tgt %d%% %dx $%d", best.target, best.leverage, best.marginUSD)
	r.Best = bestFromReport(best.report, label, best.target)
	r.Best.Leverage = best.leverage
	r.Best.MarginUSD = best.marginUSD
	attachTrades(&r, best.report)
	persistReport(w, r, candles)
}

func (u *ScanMarkets) saveFibV2Report(w *scanreport.Writer, algo algoKind, symbol string, p periodSpec, candles []model.Candle, res *fibPullbackV2SweepResult) {
	if w == nil || res == nil {
		return
	}
	r := reportBase(string(algo), symbol, p, candles)
	for _, row := range res.rows {
		r.Sweep = append(r.Sweep, sweepRowFromReport(
			fmt.Sprintf("impulse %d%%", row.minImpulse), row.minImpulse, row.report))
	}
	b := res.best
	r.Best = bestFromReport(b.report, fmt.Sprintf("impulse %d%%", b.minImpulse), b.minImpulse)
	attachTrades(&r, b.report)
	persistReport(w, r, candles)
}

func (u *ScanMarkets) saveFibTrendV1Report(w *scanreport.Writer, algo algoKind, symbol string, p periodSpec, candles []model.Candle, res *fibPullbackTrendV1SweepResult) {
	if w == nil || res == nil {
		return
	}
	r := reportBase(string(algo), symbol, p, candles)
	for _, row := range res.rows {
		label := fmt.Sprintf("imp %d%% p%d %s w%d", row.minImpulse, row.pivot, row.zone.Label(), row.maxWait)
		r.Sweep = append(r.Sweep, sweepRowFromReport(label, row.minImpulse, row.report))
	}
	b := res.best
	label := fmt.Sprintf("imp %d%% p%d %s w%d", b.minImpulse, b.pivot, b.zone.Label(), b.maxWait)
	r.Best = bestFromReport(b.report, label, b.minImpulse)
	attachTrades(&r, b.report)
	persistReport(w, r, candles)
}

func (u *ScanMarkets) saveNR7TrendBreakoutV1Report(w *scanreport.Writer, algo algoKind, symbol string, p periodSpec, candles []model.Candle, res *nr7TrendBreakoutV1SweepResult) {
	if w == nil || res == nil {
		return
	}
	r := reportBase(string(algo), symbol, p, candles)
	for _, row := range res.rows {
		label := fmt.Sprintf("NR%d ATR×%.1f L%d %s", row.nrLength, row.compression, row.lifetime, row.trendFilter.Label())
		r.Sweep = append(r.Sweep, sweepRowFromReport(label, row.nrLength, row.report))
	}
	b := res.best
	label := fmt.Sprintf("NR%d ATR×%.1f L%d %s", b.nrLength, b.compression, b.lifetime, b.trendFilter.Label())
	r.Best = bestFromReport(b.report, label, b.nrLength)
	attachTrades(&r, b.report)
	persistReport(w, r, candles)
}

func (u *ScanMarkets) saveVolatilityCompressionBreakoutV1Report(w *scanreport.Writer, algo algoKind, symbol string, p periodSpec, candles []model.Candle, res *volatilityCompressionBreakoutV1SweepResult) {
	if w == nil || res == nil {
		return
	}
	r := reportBase(string(algo), symbol, p, candles)
	for _, row := range res.rows {
		label := fmt.Sprintf("C%d R%d ATR×%.1f E%.1f %s", row.compWindow, row.rangeWindow, row.atrComp, row.expansion, row.trendFilter.Label())
		r.Sweep = append(r.Sweep, sweepRowFromReport(label, row.compWindow, row.report))
	}
	b := res.best
	label := fmt.Sprintf("C%d R%d ATR×%.1f E%.1f %s", b.compWindow, b.rangeWindow, b.atrComp, b.expansion, b.trendFilter.Label())
	r.Best = bestFromReport(b.report, label, b.compWindow)
	attachTrades(&r, b.report)
	persistReport(w, r, candles)
}

func manifestOptions(opts ScanMarketsOptions) map[string]interface{} {
	algos := make([]string, len(opts.Algos))
	for i, a := range opts.Algos {
		algos[i] = string(a)
	}
	return map[string]interface{}{
		"data_dir":       opts.Dir,
		"symbol":         opts.Symbol,
		"seed":           opts.Seed,
		"last_years":     opts.LastYears,
		"target_min":     opts.TargetMin,
		"target_max":     opts.TargetMax,
		"target_step":    opts.TargetStep,
		"min_trades":     opts.MinTrades,
		"algos":          algos,
		"sma_report":     opts.SMAReport,
		"retest_epsilon": opts.RetestEpsilonPct,
		"retest_lookahead": opts.RetestLookaheadCandles,
	}
}

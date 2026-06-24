package usecase

import (
	"context"
	"fmt"
	"hash/fnv"
	"path/filepath"
	"sort"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/infrastructure/csvdata"
	"github.com/drybin/fear-and-greed/internal/infrastructure/scanreport"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

type IScanMarkets interface {
	Process(ctx context.Context, opts ScanMarketsOptions) error
}

type ScanMarketsOptions struct {
	Dir        string
	Seed       int64
	LastYears  float64
	TargetMin  int
	TargetMax  int
	TargetStep int
	MinTrades  int
	// Retest params for trend-long-sma-retest.
	RetestEpsilonPct       float64
	RetestLookaheadCandles int
	// Margin/leverage sweep (algo drop-margin only)
	LeverageMin int
	LeverageMax int
	MarginUSDs  []int // fixed margin sizes to sweep ($)
	// Algos: empty = both; or "rise", "drop", "drop-margin"
	Algos []algoKind
	// Symbol filters to one market (e.g. BTCUSDT); empty = all CSV in dir.
	Symbol string
	// SMAReport: "best" (default) or "all" — print best target per SMA for SMA sweep algos.
	SMAReport string
	// ReportDir: if set, write JSON under <dir>/data/<algo>/ and update manifest.json.
	ReportDir string
	// HTMLPath: if set, generate comparison HTML (use "true" for reports/report.html).
	HTMLPath string
}

type periodKind string

const (
	periodFull        periodKind = "full"
	periodLast2Y      periodKind = "last_2_years"
	periodCurrentYear periodKind = "current_year"
)

type algoKind string

const (
	algoSellOnRise         algoKind = "rise"
	algoRise2DProfit       algoKind = "rise-2d-profit"
	algoSellOnDrop         algoKind = "drop"
	algoShortMarginSweep   algoKind = "drop-margin"
	algoTrend              algoKind = "trend"
	algoTrendLong          algoKind = "trend-long"
	algoTrendLongSMA       algoKind = "trend-long-sma"
	algoTrendLongSMARetest algoKind = "trend-long-sma-retest"
	algoCRTLong            algoKind = "crt-long"
	algoBreakoutRetestLong   algoKind = "breakout-retest-long"
	algoBreakoutRetestLongV2 algoKind = "breakout-retest-long-v2"
	algoFibPullbackLong      algoKind = "fib-pullback-long"
	algoFibPullbackLongV2    algoKind = "fib-pullback-long-v2"
	algoFibPullbackTrendV1   algoKind = "fib-pullback-trend-v1"
	algoNR7TrendBreakoutV1   algoKind = "nr7-trend-breakout-v1"
	algoVolatilityCompressionBreakoutV1 algoKind = "volatility-compression-breakout-v1"
	algoLiquiditySweepLong   algoKind = "liquidity-sweep-long"
	algoLiquiditySweepLongV2 algoKind = "liquidity-sweep-long-v2"
	algoLiquiditySweepLongV3 algoKind = "liquidity-sweep-long-v3"
	algoLiquiditySweepLongV4 algoKind = "liquidity-sweep-long-v4"
	algoLiquiditySweepLongV5 algoKind = "liquidity-sweep-long-v5"
)

type periodSpec struct {
	kind  periodKind
	title string
}

type runScore struct {
	symbol       string
	period       periodKind
	algo         algoKind
	sellTarget   int
	shortTarget  int // trend: short take-profit %
	smaPeriod    int // trend-long-sma
	leverage     int
	marginUSD    int
	profitPct    float64
	profitUSD    float64
	tradeCount   int
	liquidations int
	bankrupt     bool
}

type sweepRow struct {
	target int
	report strategy.SimulationReport
}

type ScanMarkets struct{}

func NewScanMarketsUsecase() *ScanMarkets {
	return &ScanMarkets{}
}

func (u *ScanMarkets) Process(_ context.Context, opts ScanMarketsOptions) error {
	if opts.TargetStep <= 0 {
		opts.TargetStep = 1
	}
	if opts.TargetMin <= 0 {
		opts.TargetMin = 1
	}
	if opts.TargetMax < opts.TargetMin {
		opts.TargetMax = opts.TargetMin
	}
	if opts.MinTrades < 1 {
		opts.MinTrades = 1
	}
	if opts.RetestEpsilonPct < 0 {
		opts.RetestEpsilonPct = strategy.TrendRetestEpsilonPct
	}
	if opts.RetestLookaheadCandles < 1 {
		opts.RetestLookaheadCandles = strategy.TrendRetestLookaheadCandles
	}
	opts.Symbol = normalizeSymbolFilter(opts.Symbol)
	opts.SMAReport = normalizeSMAReport(opts.SMAReport)
	normalizeMarginLeverageOpts(&opts)

	files, err := csvdata.ListCSVFiles(opts.Dir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no csv files in %s (run fetch-data first)", opts.Dir)
	}
	sort.Strings(files)

	periods := buildPeriods(opts)
	var scores []runScore

	algos := opts.Algos
	if len(algos) == 0 {
		algos = []algoKind{algoSellOnRise, algoSellOnDrop}
	}

	fmt.Println("=== scan-markets ===")
	fmt.Printf("data dir: %s | seed: %d | target sweep: %d%%..%d%% step %d\n",
		opts.Dir, opts.Seed, opts.TargetMin, opts.TargetMax, opts.TargetStep)
	if opts.Symbol != "" {
		fmt.Printf("symbol filter: %s\n", opts.Symbol)
	}
	if opts.SMAReport == smaReportAll {
		fmt.Println("sma-report: all (best target per SMA)")
	}
	for _, a := range algos {
		fmt.Printf("  - %s\n", algoDescription(a))
	}
	if opts.ReportDir != "" {
		fmt.Printf("  reports: JSON → %s/data/<algo>/", opts.ReportDir)
		if opts.HTMLPath != "" {
			fmt.Printf(" | HTML → %s", opts.HTMLPath)
		}
		fmt.Println()
	}
	if hasAlgo(algos, algoShortMarginSweep) {
		fmt.Printf("  drop-margin: leverage %dx | margin $%d | target sweep %d%%..%d%%\n",
			DefaultDropMarginLeverage, DefaultDropMarginUSD, opts.TargetMin, opts.TargetMax)
	}
	if hasAlgo(algos, algoTrend) {
		longMax := opts.TargetMax
		if longMax > trendLongTargetMax {
			longMax = trendLongTargetMax
		}
		shortMax := opts.TargetMax
		if shortMax > trendShortTargetMax {
			shortMax = trendShortTargetMax
		}
		fmt.Printf("  trend: SMA(%d) 1d | long +%d..%d%% × short -%d..-%d%% sweep\n",
			strategy.TrendSMAPeriod, opts.TargetMin, longMax, opts.TargetMin, shortMax)
	}
	if hasAlgo(algos, algoTrendLong) {
		fmt.Printf("  trend-long: SMA(%d) 1d | long +%d..%d%% sweep (long only)\n",
			strategy.TrendSMAPeriod, opts.TargetMin, opts.TargetMax)
	}
	if hasAlgo(algos, algoTrendLongSMA) {
		fmt.Printf("  trend-long-sma: SMA %d..%d step %d | long +%d..%d%% sweep\n",
			trendLongSMAMin, trendLongSMAMax, trendLongSMAStep, opts.TargetMin, opts.TargetMax)
	}
	if hasAlgo(algos, algoTrendLongSMARetest) {
		fmt.Printf("  trend-long-sma-retest: close breakout > SMA, retest eps %.3f%% in <=%d candles | SMA %d..%d step %d | long +%d..%d%% sweep\n",
			opts.RetestEpsilonPct, opts.RetestLookaheadCandles, trendLongSMAMin, trendLongSMAMax, trendLongSMAStep, opts.TargetMin, opts.TargetMax)
	}
	if hasAlgo(algos, algoCRTLong) {
		fmt.Printf("  crt-long: 4H impulse + 15M discount entry | TP1 50%% @ RangeHigh | TP2 swing/RR%.1f | fixed ATR×%.1f Vol×%.1f\n",
			strategy.CRTRRFallback, strategy.CRTATRMult, strategy.CRTVolMult)
	}
	if hasAlgo(algos, algoBreakoutRetestLong) {
		fmt.Printf("  breakout-retest-long: 15M swing(N=%d) break → retest zone → TP1/TP2 CRT-style | SL level-ATR×%.1f\n",
			strategy.BRSwingPivot, strategy.BRSLATRMult)
	}
	if hasAlgo(algos, algoBreakoutRetestLongV2) {
		fmt.Printf("  breakout-retest-long-v2: EMA200 1H+15M | swing N=%d | vol×%.1f | retest %d-%d | TP 1R/2R\n",
			strategy.BRV2SwingPivot, strategy.BRV2VolMult, strategy.BRV2RetestMinBars, strategy.BRV2RetestMaxBars)
	}
	if hasAlgo(algos, algoFibPullbackLong) {
		fmt.Printf("  fib-pullback-long: 1H BOS swing N=%d | fib 0.5-0.618 | min impulse %.0f%% | EMA200 rising | TP 1R/2R\n",
			strategy.FPLSwingPivot, strategy.FPLMinImpulsePct)
	}
	if hasAlgo(algos, algoFibPullbackLongV2) {
		fmt.Printf("  fib-pullback-long-v2: BOS+ATR×%.1f vol×%.1f | min impulse sweep 6/8/10%% | TP1 impulseHigh trail EMA20 | maxRisk %.1f%%\n",
			strategy.FPL2BOSATRMult, strategy.FPL2VolMult, strategy.FPL2MaxRiskPct*100)
	}
	if hasAlgo(algos, algoFibPullbackTrendV1) {
		fmt.Printf("  fib-pullback-trend-v1: spec 1H BOS (last swing high) | fib retest 15M | SL fib786 | sweep impulse/pivot/zone/wait\n")
	}
	if hasAlgo(algos, algoNR7TrendBreakoutV1) {
		fmt.Printf("  nr7-trend-breakout-v1: 1H NR compression + vol breakout | SL nrLow | sweep NR/ATR/lifetime/trend\n")
	}
	if hasAlgo(algos, algoVolatilityCompressionBreakoutV1) {
		fmt.Printf("  volatility-compression-breakout-v1: 1H ATR-min compression + range breakout | SL compLow | sweep comp/range/ATR/expansion/trend\n")
	}
	if hasAlgo(algos, algoLiquiditySweepLong) {
		fmt.Printf("  liquidity-sweep-long: 1H lowest(%d) sweep + EMA200 | confirm bar | SL sweep low | TP %.0fR\n",
			strategy.LSLLookback, strategy.LSLRRMultiple)
	}
	if hasAlgo(algos, algoLiquiditySweepLongV2) {
		fmt.Printf("  liquidity-sweep-long-v2: 1H swing pivot N=%d (max age %d) sweep + EMA200 | confirm | SL sweep low | TP %.0fR\n",
			strategy.LSL2SwingPivot, strategy.LSL2MaxSwingAge, strategy.LSL2RRMultiple)
	}
	if hasAlgo(algos, algoLiquiditySweepLongV3) {
		fmt.Printf("  liquidity-sweep-long-v3: 1H equal lows pivot tol %.1f%% sep<=%d | EMA200 | confirm | SL sweep low | TP %.0fR\n",
			strategy.LSL3EqualTolPct, strategy.LSL3MaxPoolSeparation, strategy.LSL3RRMultiple)
	}
	if hasAlgo(algos, algoLiquiditySweepLongV4) {
		fmt.Printf("  liquidity-sweep-long-v4: 1H swing sweep + displacement (ATR14×%.1f, %d bars) | EMA200 | SL sweep low | TP %.0fR\n",
			strategy.LSL4DispATRMult, strategy.LSL4DispMaxBars, strategy.LSL4RRMultiple)
	}
	if hasAlgo(algos, algoLiquiditySweepLongV5) {
		fmt.Printf("  liquidity-sweep-long-v5: 1H swing sweep + displacement FVG (≤%d bars) + retest (≤%d bars) | EMA200 | SL sweep low | TP %.0fR\n",
			strategy.LSL5ImpulseMaxBars, strategy.LSL5RetestMaxBars, strategy.LSL5RRMultiple)
	}
	fmt.Println()

	var reportWriter *scanreport.Writer
	if opts.ReportDir != "" {
		w, err := scanreport.NewWriter(opts.ReportDir)
		if err != nil {
			return fmt.Errorf("report dir: %w", err)
		}
		reportWriter = w
		for _, a := range algos {
			if err := w.ClearAlgo(string(a)); err != nil {
				return fmt.Errorf("clear report algo %s: %w", a, err)
			}
		}
	}

	matchedSymbol := false
	for _, path := range files {
		symbol := csvdata.SymbolFromFilename(filepath.Base(path))
		if !symbolMatches(symbol, opts.Symbol) {
			continue
		}
		matchedSymbol = true
		candles, err := csvdata.LoadKlines(path)
		if err != nil {
			fmt.Printf("--- %s: SKIP (%v)\n\n", symbol, err)
			continue
		}
		if len(candles) < 2 {
			fmt.Printf("--- %s: SKIP (not enough candles)\n\n", symbol)
			continue
		}

		fmt.Printf("######## %s (%d candles, %s — %s) ########\n",
			symbol,
			len(candles),
			candles[0].OpenTime.Format("2006-01-02"),
			candles[len(candles)-1].OpenTime.Format("2006-01-02"),
		)

		for _, p := range periods {
			subset, ok := subsetForPeriod(candles, p.kind, opts.LastYears)
			if !ok || len(subset) < 10 {
				fmt.Printf("--- %s | %s — SKIP (only %d candles)\n\n", symbol, p.title, len(subset))
				continue
			}

			for _, kind := range algos {
				switch kind {
				case algoShortMarginSweep:
					if best := runMarginLeverageSweep(subset, symbol, string(p.kind), opts); best != nil {
						printMarginSweepBest(symbol, p.title, subset, best, opts.MinTrades)
						u.saveMarginReport(reportWriter, kind, symbol, p, subset, best)
						if best.report.CompletedCount >= opts.MinTrades {
							scores = append(scores, marginScore(symbol, p.kind, best))
						}
					} else {
						fmt.Printf("--- %s | %s | %s — no runs\n\n",
							symbol, p.title, algoSweepTitle(kind))
					}
				case algoTrend:
					if best := runTrendSweep(subset, symbol, string(p.kind), opts); best != nil {
						printTrendSweepBest(symbol, p.title, subset, best, opts.MinTrades)
						u.saveTrendReport(reportWriter, kind, symbol, p, subset, best)
						if best.report.CompletedCount >= opts.MinTrades {
							scores = append(scores, trendScore(symbol, p.kind, best))
						}
					} else {
						fmt.Printf("--- %s | %s | %s — no runs\n\n",
							symbol, p.title, algoSweepTitle(kind))
					}
				case algoTrendLong:
					if best := runTrendLongSweep(subset, symbol, string(p.kind), opts); best != nil {
						printTrendLongSweepBest(symbol, p.title, subset, best, opts.MinTrades)
						u.saveTrendLongReport(reportWriter, kind, symbol, p, subset, best)
						if best.report.CompletedCount >= opts.MinTrades {
							scores = append(scores, trendLongScore(symbol, p.kind, best))
						}
					} else {
						fmt.Printf("--- %s | %s | %s — no runs\n\n",
							symbol, p.title, algoSweepTitle(kind))
					}
				case algoTrendLongSMA:
					if res := runTrendLongSMASweep(subset, symbol, string(p.kind), opts); res != nil {
						printTrendLongSMASweepResult(symbol, p.title, subset, res, opts.MinTrades, opts.SMAReport)
						u.saveTrendLongSMAReport(reportWriter, kind, symbol, p, subset, res)
						if res.best.report.CompletedCount >= opts.MinTrades {
							scores = append(scores, trendLongSMAScore(symbol, p.kind, res.best))
						}
					} else {
						fmt.Printf("--- %s | %s | %s — no runs\n\n",
							symbol, p.title, algoSweepTitle(kind))
					}
				case algoTrendLongSMARetest:
					if res := runTrendLongSMARetestSweep(subset, symbol, string(p.kind), opts); res != nil {
						printTrendLongSMARetestSweepResult(symbol, p.title, subset, res, opts.MinTrades, opts.SMAReport, opts.RetestEpsilonPct, opts.RetestLookaheadCandles)
						u.saveTrendLongSMARetestReport(reportWriter, kind, symbol, p, subset, res)
						if res.best.report.CompletedCount >= opts.MinTrades {
							scores = append(scores, trendLongSMARetestScore(symbol, p.kind, res.best))
						}
					} else {
						fmt.Printf("--- %s | %s | %s — no runs\n\n",
							symbol, p.title, algoSweepTitle(kind))
					}
				case algoCRTLong:
					rep := runCRTLong(subset)
					printCRTLongBest(symbol, p.title, subset, rep, opts.MinTrades)
					u.saveSimReport(reportWriter, kind, symbol, p, subset, rep, "fixed", 0)
					if rep.CompletedCount >= opts.MinTrades || rep.OpenPosition {
						scores = append(scores, crtLongScore(symbol, p.kind, rep))
					}
				case algoBreakoutRetestLong:
					rep := runBreakoutRetestLong(subset)
					printBreakoutRetestLongBest(symbol, p.title, subset, rep, opts.MinTrades)
					u.saveSimReport(reportWriter, kind, symbol, p, subset, rep, "fixed", 0)
					if rep.CompletedCount >= opts.MinTrades || rep.OpenPosition {
						scores = append(scores, breakoutRetestLongScore(symbol, p.kind, rep))
					}
				case algoBreakoutRetestLongV2:
					rep := runBreakoutRetestLongV2(subset)
					printBreakoutRetestLongV2Best(symbol, p.title, subset, rep, opts.MinTrades)
					u.saveSimReport(reportWriter, kind, symbol, p, subset, rep, "fixed", 0)
					if rep.CompletedCount >= opts.MinTrades || rep.OpenPosition {
						scores = append(scores, breakoutRetestLongV2Score(symbol, p.kind, rep))
					}
				case algoFibPullbackLong:
					rep := runFibPullbackLong(subset)
					printFibPullbackLongBest(symbol, p.title, subset, rep, opts.MinTrades)
					u.saveSimReport(reportWriter, kind, symbol, p, subset, rep, "fixed", 0)
					if rep.CompletedCount >= opts.MinTrades || rep.OpenPosition {
						scores = append(scores, fibPullbackLongScore(symbol, p.kind, rep))
					}
				case algoFibPullbackLongV2:
					res := runFibPullbackLongV2Sweep(subset)
					printFibPullbackLongV2Sweep(symbol, p.title, subset, res, opts.MinTrades)
					u.saveFibV2Report(reportWriter, kind, symbol, p, subset, res)
					if res != nil {
						scores = append(scores, fibPullbackLongV2Score(symbol, p.kind, res))
					}
				case algoFibPullbackTrendV1:
					res := runFibPullbackTrendV1Sweep(subset)
					printFibPullbackTrendV1Sweep(symbol, p.title, subset, res, opts.MinTrades)
					u.saveFibTrendV1Report(reportWriter, kind, symbol, p, subset, res)
					if res != nil && (res.best.report.CompletedCount >= opts.MinTrades || res.best.report.OpenPosition) {
						scores = append(scores, fibPullbackTrendV1Score(symbol, p.kind, res))
					}
				case algoNR7TrendBreakoutV1:
					res := runNR7TrendBreakoutV1Sweep(subset)
					printNR7TrendBreakoutV1Sweep(symbol, p.title, subset, res, opts.MinTrades)
					u.saveNR7TrendBreakoutV1Report(reportWriter, kind, symbol, p, subset, res)
					if res != nil && (res.best.report.CompletedCount >= opts.MinTrades || res.best.report.OpenPosition) {
						scores = append(scores, nr7TrendBreakoutV1Score(symbol, p.kind, res))
					}
				case algoVolatilityCompressionBreakoutV1:
					res := runVolatilityCompressionBreakoutV1Sweep(subset)
					printVolatilityCompressionBreakoutV1Sweep(symbol, p.title, subset, res, opts.MinTrades)
					u.saveVolatilityCompressionBreakoutV1Report(reportWriter, kind, symbol, p, subset, res)
					if res != nil && (res.best.report.CompletedCount >= opts.MinTrades || res.best.report.OpenPosition) {
						scores = append(scores, volatilityCompressionBreakoutV1Score(symbol, p.kind, res))
					}
				case algoLiquiditySweepLong:
					rep := runLiquiditySweepLong(subset)
					printLiquiditySweepLongBest(symbol, p.title, subset, rep, opts.MinTrades)
					u.saveSimReport(reportWriter, kind, symbol, p, subset, rep, "fixed", 0)
					if rep.CompletedCount >= opts.MinTrades || rep.OpenPosition {
						scores = append(scores, liquiditySweepLongScore(symbol, p.kind, rep))
					}
				case algoLiquiditySweepLongV2:
					rep := runLiquiditySweepLongV2(subset)
					printLiquiditySweepLongV2Best(symbol, p.title, subset, rep, opts.MinTrades)
					u.saveSimReport(reportWriter, kind, symbol, p, subset, rep, "fixed", 0)
					if rep.CompletedCount >= opts.MinTrades || rep.OpenPosition {
						scores = append(scores, liquiditySweepLongV2Score(symbol, p.kind, rep))
					}
				case algoLiquiditySweepLongV3:
					rep := runLiquiditySweepLongV3(subset)
					printLiquiditySweepLongV3Best(symbol, p.title, subset, rep, opts.MinTrades)
					u.saveSimReport(reportWriter, kind, symbol, p, subset, rep, "fixed", 0)
					if rep.CompletedCount >= opts.MinTrades || rep.OpenPosition {
						scores = append(scores, liquiditySweepLongV3Score(symbol, p.kind, rep))
					}
				case algoLiquiditySweepLongV4:
					rep := runLiquiditySweepLongV4(subset)
					printLiquiditySweepLongV4Best(symbol, p.title, subset, rep, opts.MinTrades)
					u.saveSimReport(reportWriter, kind, symbol, p, subset, rep, "fixed", 0)
					if rep.CompletedCount >= opts.MinTrades || rep.OpenPosition {
						scores = append(scores, liquiditySweepLongV4Score(symbol, p.kind, rep))
					}
				case algoLiquiditySweepLongV5:
					rep := runLiquiditySweepLongV5(subset)
					printLiquiditySweepLongV5Best(symbol, p.title, subset, rep, opts.MinTrades)
					u.saveSimReport(reportWriter, kind, symbol, p, subset, rep, "fixed", 0)
					if rep.CompletedCount >= opts.MinTrades || rep.OpenPosition {
						scores = append(scores, liquiditySweepLongV5Score(symbol, p.kind, rep))
					}
				default:
					rows := runTargetSweep(subset, symbol, string(p.kind), kind, opts)
					printSweepTable(symbol, p.title, algoSweepTitle(kind), subset, rows, opts.MinTrades)
					if best := bestSweepRow(rows, opts.MinTrades); best != nil {
						u.saveSweepReport(reportWriter, kind, symbol, p, subset, rows, best)
						scores = append(scores, runScore{
							symbol:     symbol,
							period:     p.kind,
							algo:       kind,
							sellTarget: best.target,
							profitPct:  best.report.ProfitPct,
							profitUSD:  best.report.ProfitUSD,
							tradeCount: best.report.CompletedCount,
						})
					}
				}
			}
		}
		fmt.Println()
	}

	if opts.Symbol != "" && !matchedSymbol {
		return fmt.Errorf("symbol %q not found in %s", opts.Symbol, opts.Dir)
	}

	printSummary(scores)

	if reportWriter != nil {
		if err := reportWriter.FinishManifest(manifestOptions(opts)); err != nil {
			return fmt.Errorf("write manifest: %w", err)
		}
		fmt.Printf("\n=== reports written: %s ===\n", opts.ReportDir)
		if opts.HTMLPath != "" {
			if err := scanreport.GenerateHTML(opts.ReportDir, opts.HTMLPath); err != nil {
				return fmt.Errorf("generate html: %w", err)
			}
			fmt.Printf("HTML report: %s\n", opts.HTMLPath)
			fmt.Printf("Chart page:  %s\n", scanreport.DefaultChartHTMLPath(opts.ReportDir))
		}
	}

	return nil
}

func buildPeriods(opts ScanMarketsOptions) []periodSpec {
	return []periodSpec{
		{kind: periodFull, title: "весь период"},
		{kind: periodLast2Y, title: fmt.Sprintf("последние %.0f года", opts.LastYears)},
		{kind: periodCurrentYear, title: "текущий год (год последней свечи)"},
	}
}

func subsetForPeriod(candles []model.Candle, kind periodKind, lastYears float64) ([]model.Candle, bool) {
	switch kind {
	case periodFull:
		return candles, true
	case periodLast2Y:
		return strategy.FilterLastYears(candles, lastYears), true
	case periodCurrentYear:
		return strategy.FilterCurrentYear(candles), true
	default:
		return nil, false
	}
}

func runTargetSweep(
	candles []model.Candle,
	symbol, period string,
	algo algoKind,
	opts ScanMarketsOptions,
) []sweepRow {
	var rows []sweepRow
	for target := opts.TargetMin; target <= opts.TargetMax; target += opts.TargetStep {
		seed := seedFor(symbol, period, string(algo), target, opts.Seed)
		var rep strategy.SimulationReport
		switch algo {
		case algoSellOnRise:
			rep = strategy.SimulateRandomTarget(candles, seed, float64(target))
		case algoRise2DProfit:
			rep = strategy.SimulateRandomRise2DayProfit(candles, seed, float64(target))
		case algoSellOnDrop:
			rep = strategy.SimulateRandomTargetDrop(candles, seed, float64(target))
		}
		rows = append(rows, sweepRow{target: target, report: rep})
	}
	return rows
}

// bestSweepRow picks max profit among runs with at least minTrades completed round-trips.
// OpenPosition at end (last leg still open) is OK — earlier trades still count.
func bestSweepRow(rows []sweepRow, minTrades int) *sweepRow {
	var best *sweepRow
	for i := range rows {
		r := &rows[i]
		if r.report.CompletedCount < minTrades {
			continue
		}
		if best == nil || r.report.ProfitPct > best.report.ProfitPct {
			best = r
		}
	}
	return best
}

func seedFor(symbol, period, algo string, targetPct int, base int64) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(symbol))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(period))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(algo))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(fmt.Sprintf("%d", targetPct)))
	return int64(h.Sum64()) ^ base
}

func printSweepTable(symbol, periodTitle, algoTitle string, candles []model.Candle, rows []sweepRow, minTrades int) {
	from := candles[0].OpenTime.Format("2006-01-02 15:04")
	to := candles[len(candles)-1].OpenTime.Format("2006-01-02 15:04")

	fmt.Printf("--- %s | %s | %s ---\n", symbol, periodTitle, algoTitle)
	fmt.Printf("    range: %s → %s (%d candles)\n", from, to, len(candles))
	fmt.Println("    (profit %/$ = closed trades only; open leg at end shown separately if any)")
	fmt.Printf("    %6s  %7s  %10s  %10s  %12s\n", "target", "trades", "profit %", "profit $", "wait avg h")
	for _, row := range rows {
		avgWait := "n/a"
		if row.report.CompletedCount > 0 {
			avgWait = formatHours(row.report.WaitHoursAvg)
		}
		fmt.Printf("    %5d%%  %7d  %10s  %10.2f  %12s\n",
			row.target,
			row.report.CompletedCount,
			formatProfitPct(row.report),
			formatProfitUSD(row.report),
			avgWait,
		)
	}
	if best := bestSweepRow(rows, minTrades); best != nil {
		fmt.Printf("    >> best target: %d%%  profit %+.2f%%  ($%.2f, %d trades)%s\n\n",
			best.target, best.report.ProfitPct, best.report.ProfitUSD, best.report.CompletedCount,
			formatOpenLegNote(best.report))
	} else {
		fmt.Printf("    >> best target: n/a (no completed trades in sweep; try lower %% for drop)\n\n")
	}
}

func formatProfitPct(rep strategy.SimulationReport) string {
	if rep.CompletedCount > 0 {
		return fmt.Sprintf("%+.2f%%", rep.ProfitPct)
	}
	if rep.OpenPosition {
		return "no exit"
	}
	return "n/a"
}

func formatOpenLegNote(rep strategy.SimulationReport) string {
	if !rep.OpenPosition || rep.OpenLegUSD == 0 {
		return ""
	}
	return fmt.Sprintf("  [open leg $%+.2f MTM]", rep.OpenLegUSD)
}

func formatProfitUSD(rep strategy.SimulationReport) float64 {
	if rep.CompletedCount > 0 || rep.OpenPosition {
		return rep.ProfitUSD
	}
	return 0
}

func formatHours(h float64) string {
	return fmt.Sprintf("%.1f", h)
}

func printSummary(scores []runScore) {
	if len(scores) == 0 {
		fmt.Println("=== summary: no results ===")
		return
	}

	fmt.Println("=== summary: best coin (best target % per symbol+period+algo) ===")

	algosSeen := uniqueAlgos(scores)
	for _, algo := range algosSeen {
		fmt.Printf("  [%s]\n", algoLabel(algo))
		for _, p := range []periodKind{periodFull, periodLast2Y, periodCurrentYear} {
			best := bestByPeriodAlgo(scores, p, algo)
			if best != nil {
				fmt.Printf("    %-14s  %s%s\n",
					periodLabel(p), best.symbol, formatScoreLine(*best))
			}
		}
	}

	sorted := append([]runScore(nil), scores...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].profitPct > sorted[j].profitPct
	})
	top := sorted[0]
	fmt.Printf("  overall best:    %s [%s/%s]%s\n",
		top.symbol, algoLabel(top.algo), periodLabel(top.period), formatScoreLine(top))

	fmt.Println("\n  all symbols (best target with >=1 completed trade):")
	sortedAll := append([]runScore(nil), scores...)
	sort.Slice(sortedAll, func(i, j int) bool {
		if sortedAll[i].algo != sortedAll[j].algo {
			return sortedAll[i].algo < sortedAll[j].algo
		}
		if sortedAll[i].period != sortedAll[j].period {
			return sortedAll[i].period < sortedAll[j].period
		}
		return sortedAll[i].profitPct > sortedAll[j].profitPct
	})
	for _, s := range sortedAll {
		fmt.Printf("    %-20s %-12s %-5s%s\n",
			s.symbol, periodLabel(s.period), algoLabel(s.algo), formatScoreLine(s))
	}
}

func bestByPeriodAlgo(scores []runScore, p periodKind, algo algoKind) *runScore {
	var best *runScore
	for i := range scores {
		if scores[i].period != p || scores[i].algo != algo {
			continue
		}
		if best == nil || scores[i].profitPct > best.profitPct {
			best = &scores[i]
		}
	}
	return best
}

func periodLabel(p periodKind) string {
	switch p {
	case periodFull:
		return "full"
	case periodLast2Y:
		return "last_2y"
	case periodCurrentYear:
		return "current_year"
	default:
		return string(p)
	}
}

// ParseAlgos parses --algo flag values: rise, drop, all (default all = both).
func ParseAlgos(values []string) ([]algoKind, error) {
	if len(values) == 0 {
		return nil, nil
	}
	var out []algoKind
	for _, v := range values {
		switch v {
		case "all":
			return []algoKind{algoSellOnRise, algoSellOnDrop}, nil
		case "rise", "up", "buy-rise":
			out = append(out, algoSellOnRise)
		case "rise-2d-profit", "rise-2d", "rise-time-profit":
			out = append(out, algoRise2DProfit)
		case "drop", "down", "sell-drop":
			out = append(out, algoSellOnDrop)
		case "drop-margin", "margin", "leveraged", "short-margin":
			out = append(out, algoShortMarginSweep)
		case "trend", "adaptive", "sma":
			out = append(out, algoTrend)
		case "trend-long", "trendlong", "bull-long":
			out = append(out, algoTrendLong)
		case "trend-long-sma", "trend-long-sma-sweep":
			out = append(out, algoTrendLongSMA)
		case "trend-long-sma-retest", "trend-long-retest", "sma-retest":
			out = append(out, algoTrendLongSMARetest)
		case "crt-long", "crt":
			out = append(out, algoCRTLong)
		case "breakout-retest-long", "breakout-retest", "br-retest":
			out = append(out, algoBreakoutRetestLong)
		case "breakout-retest-long-v2", "breakout-retest-v2", "br-retest-v2":
			out = append(out, algoBreakoutRetestLongV2)
		case "fib-pullback-long", "fib-pullback", "fib-long":
			out = append(out, algoFibPullbackLong)
		case "fib-pullback-long-v2", "fib-pullback-v2", "fib-v2":
			out = append(out, algoFibPullbackLongV2)
		case "fib-pullback-trend-v1", "fib-trend-v1", "fpt-v1":
			out = append(out, algoFibPullbackTrendV1)
		case "nr7-trend-breakout-v1", "nr7-breakout", "nr7", "nr-breakout":
			out = append(out, algoNR7TrendBreakoutV1)
		case "volatility-compression-breakout-v1", "vol-compression-breakout", "vcb-v1", "vcb":
			out = append(out, algoVolatilityCompressionBreakoutV1)
		case "liquidity-sweep-long", "liquidity-sweep", "sweep-long", "lsl":
			out = append(out, algoLiquiditySweepLong)
		case "liquidity-sweep-long-v2", "liquidity-sweep-v2", "sweep-long-v2", "lsl-v2":
			out = append(out, algoLiquiditySweepLongV2)
		case "liquidity-sweep-long-v3", "liquidity-sweep-v3", "sweep-long-v3", "lsl-v3", "equal-lows":
			out = append(out, algoLiquiditySweepLongV3)
		case "liquidity-sweep-long-v4", "liquidity-sweep-v4", "sweep-long-v4", "lsl-v4", "displacement":
			out = append(out, algoLiquiditySweepLongV4)
		case "liquidity-sweep-long-v5", "liquidity-sweep-v5", "sweep-long-v5", "lsl-v5", "fvg":
			out = append(out, algoLiquiditySweepLongV5)
		default:
			return nil, fmt.Errorf("unknown algo %q (use rise, rise-2d-profit, drop, drop-margin, trend, trend-long, trend-long-sma, trend-long-sma-retest, crt-long, breakout-retest-long, breakout-retest-long-v2, fib-pullback-long, fib-pullback-long-v2, fib-pullback-trend-v1, nr7-trend-breakout-v1, volatility-compression-breakout-v1, liquidity-sweep-long, liquidity-sweep-long-v2, liquidity-sweep-long-v3, liquidity-sweep-long-v4, liquidity-sweep-long-v5, or all)", v)
		}
	}
	return dedupeAlgos(out), nil
}

func dedupeAlgos(in []algoKind) []algoKind {
	seen := make(map[algoKind]bool)
	var out []algoKind
	for _, a := range in {
		if seen[a] {
			continue
		}
		seen[a] = true
		out = append(out, a)
	}
	return out
}

func uniqueAlgos(scores []runScore) []algoKind {
	return dedupeAlgos(func() []algoKind {
		var a []algoKind
		for _, s := range scores {
			a = append(a, s.algo)
		}
		return a
	}())
}

func algoSweepTitle(a algoKind) string {
	switch a {
	case algoSellOnRise:
		return "sell on rise +N%"
	case algoRise2DProfit:
		return "rise 2d +2% or target +N%"
	case algoSellOnDrop:
		return "short 1x, cover on -N%"
	case algoShortMarginSweep:
		return "short + leverage/margin sweep"
	case algoTrend:
		return "SMA(50) 1d → long/short targets"
	case algoTrendLong:
		return "SMA(50) 1d → long-only target"
	case algoTrendLongSMA:
		return "SMA sweep → long-only target"
	case algoTrendLongSMARetest:
		return "SMA breakout+retest → long-only target"
	case algoCRTLong:
		return "CRT 4H impulse → 15M long (TP1/TP2)"
	case algoBreakoutRetestLong:
		return "15M swing break → retest long (TP1/TP2)"
	case algoBreakoutRetestLongV2:
		return "BR v2 EMA200 + vol + 1R/2R TP"
	case algoFibPullbackLong:
		return "1H BOS fib 0.5-0.618 pullback long"
	case algoFibPullbackLongV2:
		return "fib v2 BOS+vol impulseHigh TP sweep"
	case algoFibPullbackTrendV1:
		return "fib trend v1 spec BOS+fib786 SL sweep"
	case algoNR7TrendBreakoutV1:
		return "NR compression breakout 1H sweep"
	case algoVolatilityCompressionBreakoutV1:
		return "ATR compression range breakout 1H sweep"
	case algoLiquiditySweepLong:
		return "1H liquidity sweep long (lowest20 + confirm)"
	case algoLiquiditySweepLongV2:
		return "1H swing sweep long (pivot + confirm)"
	case algoLiquiditySweepLongV3:
		return "1H equal lows sweep long"
	case algoLiquiditySweepLongV4:
		return "1H swing sweep + displacement long"
	case algoLiquiditySweepLongV5:
		return "1H swing sweep + FVG retest long"
	default:
		return string(a)
	}
}

func algoDescription(a algoKind) string {
	switch a {
	case algoSellOnRise:
		return "rise: buy → sell when price +N%"
	case algoRise2DProfit:
		return "rise-2d: random daily long; before 48h exit at +target%; after 48h exit at +2%; no stop"
	case algoSellOnDrop:
		return "drop: short 1x → cover when price -N%"
	case algoShortMarginSweep:
		return "drop-margin: short 2x, margin $30, sweep target %, track liquidations"
	case algoTrend:
		return "trend: SMA(50) 1d bull→long / bear→short $30 1x, sweep long%×short%"
	case algoTrendLong:
		return "trend-long: SMA(50) 1d bull→long only, sweep long%"
	case algoTrendLongSMA:
		return "trend-long-sma: SMA 10..100 step 10, bull→long only, sweep long%"
	case algoTrendLongSMARetest:
		return "trend-long-sma-retest: close breakout above SMA then retest (eps, lookahead), sweep SMA×long%"
	case algoCRTLong:
		return "crt-long: 4H impulse + 15M discount, partial TP at RangeHigh, TP2 swing/RR2"
	case algoBreakoutRetestLong:
		return "breakout-retest-long: 15M swing break + retest zone, CRT-style exits"
	case algoBreakoutRetestLongV2:
		return "breakout-retest-long-v2: EMA200 1H+15M, volume filter, retest 3-12, TP 1R/2R"
	case algoFibPullbackLong:
		return "fib-pullback-long: 1H BOS + fib 0.5-0.618, EMA200 trend, 15M EMA20 entry, TP 1R/2R"
	case algoFibPullbackLongV2:
		return "fib-pullback-long-v2: stricter BOS+vol, confirm 3-bar high, TP1 impulseHigh, trail EMA20, impulse sweep 6/8/10%"
	case algoFibPullbackTrendV1:
		return "fib-pullback-trend-v1: spec 1H BOS last swing high, 15M fib retest, SL fib786, low-based exits, full param sweep"
	case algoNR7TrendBreakoutV1:
		return "nr7-trend-breakout-v1: 1H NR compression, breakout close>nrHigh+range>ATR, SL nrLow, TP 1R/2R, full param sweep"
	case algoVolatilityCompressionBreakoutV1:
		return "volatility-compression-breakout-v1: 1H ATR-min compression, range breakout+expansion, SL compLow, TP 1R/2R, full param sweep"
	case algoLiquiditySweepLong:
		return "liquidity-sweep-long: 1H sweep below 20-bar low, close>prior, EMA200, confirm close>sweep high, SL sweep low, TP 2R"
	case algoLiquiditySweepLongV2:
		return "liquidity-sweep-long-v2: 1H swing pivot low sweep, EMA200, confirm bar, SL sweep low, TP 2R"
	case algoLiquiditySweepLongV3:
		return "liquidity-sweep-long-v3: 1H equal lows pool (0.2%% tol), sweep+return, EMA200, confirm, SL sweep low, TP 2R"
	case algoLiquiditySweepLongV4:
		return "liquidity-sweep-long-v4: 1H swing sweep + bullish displacement (range>ATR14×1.5), EMA200, SL sweep low, TP 2R"
	case algoLiquiditySweepLongV5:
		return "liquidity-sweep-long-v5: 1H swing sweep + displacement FVG + retest, EMA200, SL sweep low, TP 2R"
	default:
		return string(a)
	}
}

func algoLabel(a algoKind) string {
	switch a {
	case algoSellOnRise:
		return "rise"
	case algoRise2DProfit:
		return "rise-2d"
	case algoSellOnDrop:
		return "drop"
	case algoShortMarginSweep:
		return "drop-m"
	case algoTrend:
		return "trend"
	case algoTrendLong:
		return "trend-l"
	case algoTrendLongSMA:
		return "t-l-sma"
	case algoTrendLongSMARetest:
		return "t-l-ret"
	case algoCRTLong:
		return "crt"
	case algoBreakoutRetestLong:
		return "br-ret"
	case algoBreakoutRetestLongV2:
		return "br-v2"
	case algoFibPullbackLong:
		return "fib"
	case algoFibPullbackLongV2:
		return "fib-v2"
	case algoFibPullbackTrendV1:
		return "fpt-v1"
	case algoNR7TrendBreakoutV1:
		return "nr7"
	case algoVolatilityCompressionBreakoutV1:
		return "vcb"
	case algoLiquiditySweepLong:
		return "liq-sweep"
	case algoLiquiditySweepLongV2:
		return "liq-sweep-v2"
	case algoLiquiditySweepLongV3:
		return "liq-sweep-v3"
	case algoLiquiditySweepLongV4:
		return "liq-sweep-v4"
	case algoLiquiditySweepLongV5:
		return "liq-sweep-v5"
	default:
		return string(a)
	}
}

func formatScoreLine(s runScore) string {
	switch s.algo {
	case algoShortMarginSweep:
		bk := ""
		if s.bankrupt {
			bk = " bankrupt"
		}
		return fmt.Sprintf("  tgt %d%% %dx $%d  %+.2f%% ($%.2f, %d trades, %d liq%s)",
			s.sellTarget, s.leverage, s.marginUSD, s.profitPct, s.profitUSD, s.tradeCount, s.liquidations, bk)
	case algoTrend:
		bk := ""
		if s.bankrupt {
			bk = " bankrupt"
		}
		return fmt.Sprintf("  long +%d%% short -%d%%  %+.2f%% ($%.2f, %d trades, %d liq%s)",
			s.sellTarget, s.shortTarget, s.profitPct, s.profitUSD, s.tradeCount, s.liquidations, bk)
	case algoTrendLongSMA:
		return fmt.Sprintf("  SMA(%d) long +%d%%  %+.2f%% ($%.2f, %d trades)",
			s.smaPeriod, s.sellTarget, s.profitPct, s.profitUSD, s.tradeCount)
	case algoTrendLongSMARetest:
		return fmt.Sprintf("  SMA(%d) retest long +%d%%  %+.2f%% ($%.2f, %d trades)",
			s.smaPeriod, s.sellTarget, s.profitPct, s.profitUSD, s.tradeCount)
	case algoFibPullbackLongV2:
		return fmt.Sprintf("  impulse %d%%  %+.2f%%  ($%.2f, %d trades)", s.sellTarget, s.profitPct, s.profitUSD, s.tradeCount)
	case algoFibPullbackTrendV1:
		return fmt.Sprintf("  impulse %d%% pivot %d  %+.2f%%  ($%.2f, %d trades)", s.sellTarget, s.smaPeriod, s.profitPct, s.profitUSD, s.tradeCount)
	case algoNR7TrendBreakoutV1:
		return fmt.Sprintf("  NR%d life %d  %+.2f%%  ($%.2f, %d trades)", s.sellTarget, s.smaPeriod, s.profitPct, s.profitUSD, s.tradeCount)
	case algoVolatilityCompressionBreakoutV1:
		return fmt.Sprintf("  comp%d range%d  %+.2f%%  ($%.2f, %d trades)", s.sellTarget, s.smaPeriod, s.profitPct, s.profitUSD, s.tradeCount)
	default:
		return fmt.Sprintf("  target %d%%  %+.2f%%  ($%.2f, %d trades)", s.sellTarget, s.profitPct, s.profitUSD, s.tradeCount)
	}
}

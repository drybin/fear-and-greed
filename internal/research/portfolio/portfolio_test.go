package portfolio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func TestRelativeStrengthUsesOnlyCompletedPreRebalanceBarsAndBreaksTies(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	btc := syntheticBars(start, 220, 100, 0.003)
	aaa := syntheticBars(start, 220, 50, 0.005)
	bbb := append([]DailyBar(nil), aaa...)
	bars := map[protocolv2.Symbol][]DailyBar{"BTCUSDT": btc, "BBBUSDT": bbb, "AAAUSDT": aaa}
	cfg := RelativeStrengthConfig{ReturnLookbackDays: 30, VolatilityDays: 20, ATRDays: 14, StopATR: 2, TopK: 2, ExitRank: 3, BTCEMADays: 50, MinPositiveBreadth: 0, RebalanceWeekday: time.Monday}

	var fill time.Time
	for _, bar := range btc[100:] {
		if bar.Time.Weekday() == time.Monday {
			fill = bar.Time
			break
		}
	}
	require.False(t, fill.IsZero())
	original, err := RelativeStrengthRebalances(bars, cfg, fill, fill.Add(24*time.Hour))
	require.NoError(t, err)
	require.Len(t, original, 1)
	require.Equal(t, protocolv2.Symbol("AAAUSDT"), original[0].Ranking[0].Symbol)
	require.Equal(t, protocolv2.Symbol("BBBUSDT"), original[0].Ranking[1].Symbol)

	changed := make(map[protocolv2.Symbol][]DailyBar, len(bars))
	for symbol, series := range bars {
		changed[symbol] = append([]DailyBar(nil), series...)
	}
	fillIndex := int(fill.Sub(start) / (24 * time.Hour))
	changed["AAAUSDT"][fillIndex].Close *= 100
	changed["AAAUSDT"][fillIndex].High = changed["AAAUSDT"][fillIndex].Close
	after, err := RelativeStrengthRebalances(changed, cfg, fill, fill.Add(24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, original, after, "the fill-day candle must not influence its own ranking")
}

func TestRegimeModesEnableOnlyTheirFrozenFilters(t *testing.T) {
	tests := []struct {
		name            string
		mode            RegimeMode
		btcAboveEMA     bool
		breadthPositive bool
		want            bool
	}{
		{name: "both requires both inputs", mode: RegimeModeBoth, btcAboveEMA: true, breadthPositive: false, want: false},
		{name: "btc ema ignores breadth", mode: RegimeModeBTCEMA, btcAboveEMA: true, breadthPositive: false, want: true},
		{name: "breadth ignores btc ema", mode: RegimeModeBreadth, btcAboveEMA: false, breadthPositive: true, want: true},
		{name: "none ignores both filters", mode: RegimeModeNone, btcAboveEMA: false, breadthPositive: false, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, regimeEnabled(test.mode, test.btcAboveEMA, test.breadthPositive))
		})
	}
	require.Error(t, RegimeMode("unknown").Validate())
}

func TestTrendPullbackRequiresPriceNearItsCompletedEMA(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	series := syntheticBars(start, 40, 100, .001)
	cfg := RelativeStrengthConfig{ReturnLookbackDays: 10, VolatilityDays: 5, ATRDays: 5, StopATR: 2, TopK: 1, ExitRank: 2, BTCEMADays: 10, MinPositiveBreadth: .5, RebalanceWeekday: time.Monday, EntryMode: EntryModeTrendPullback, PullbackEMADays: 5, MaxEntryDistanceATR: .5}

	near, ok := score("AAAUSDT", series, cfg)
	require.True(t, ok)
	require.True(t, near.EntryEligible)
	require.Greater(t, near.MaxEntryPrice, 0.0)

	extended := append([]DailyBar(nil), series...)
	extended[len(extended)-1].Close *= 1.10
	extended[len(extended)-1].High = extended[len(extended)-1].Close
	far, ok := score("AAAUSDT", extended, cfg)
	require.True(t, ok)
	require.False(t, far.EntryEligible)

	require.Equal(t, protocolv2.StrategyCode("relative-strength-pullback-v1"), relativeStrengthStrategy(EntryModeTrendPullback).Code)
}

func TestEngineRejectsPullbackEntryThatGapsAboveFrozenCap(t *testing.T) {
	day := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	bars := map[protocolv2.Symbol][]DailyBar{"AAAUSDT": {{Time: day, Open: 101, High: 102, Low: 99, Close: 100}}}
	event := Rebalance{FillTime: day, RegimeOn: true, Targets: []Rank{{Symbol: "AAAUSDT", Rank: 1, StopDistance: 5, EntryEligible: true, MaxEntryPrice: 100}}}
	limits := Limits{InitialCapital: 10_000, RiskPerTradePercent: 1, MaxPositionPercent: 20, MaxPositions: 1, MaxAggregateRiskPct: 5}
	result, err := (Engine{Limits: limits}).Run(bars, []Rebalance{event}, day, day.Add(24*time.Hour))
	require.NoError(t, err)
	require.Len(t, result.Decisions, 1)
	require.False(t, result.Decisions[0].Accepted)
	require.Equal(t, "entry_extension", result.Decisions[0].Reason)
}

func TestEngineEnforcesSlotsAndReconcilesCash(t *testing.T) {
	day := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	bars := map[protocolv2.Symbol][]DailyBar{}
	for _, symbol := range []protocolv2.Symbol{"AAAUSDT", "BBBUSDT", "CCCUSDT"} {
		bars[symbol] = []DailyBar{
			{Time: day, Open: 100, High: 105, Low: 95, Close: 100},
			{Time: day.Add(24 * time.Hour), Open: 100, High: 112, Low: 95, Close: 110},
		}
	}
	event := Rebalance{FillTime: day, RegimeOn: true, Retain: map[protocolv2.Symbol]bool{}, Targets: []Rank{
		{Symbol: "AAAUSDT", Rank: 1, StopDistance: 10},
		{Symbol: "BBBUSDT", Rank: 2, StopDistance: 10},
		{Symbol: "CCCUSDT", Rank: 3, StopDistance: 10},
	}}
	limits := Limits{InitialCapital: 10_000, RiskPerTradePercent: 1, MaxPositionPercent: 20, MaxPositions: 2, MaxAggregateRiskPct: 5}
	result, err := (Engine{Limits: limits}).Run(bars, []Rebalance{event}, day, day.Add(48*time.Hour))
	require.NoError(t, err)
	require.Len(t, result.Trades, 2)
	require.Len(t, result.Decisions, 3)
	require.True(t, result.Decisions[0].Accepted)
	require.True(t, result.Decisions[1].Accepted)
	require.Equal(t, "position_limit", result.Decisions[2].Reason)
	require.Equal(t, 10_200.0, result.FinalCash)
	pnl := 0.0
	for _, trade := range result.Trades {
		pnl += trade.NetPnL
	}
	require.Equal(t, result.FinalCash-result.InitialCapital, pnl)
}

func TestEngineOrdersSignalsAndEnforcesAggregateRisk(t *testing.T) {
	day := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	bars := map[protocolv2.Symbol][]DailyBar{}
	for _, symbol := range []protocolv2.Symbol{"AAAUSDT", "BBBUSDT"} {
		bars[symbol] = []DailyBar{{Time: day, Open: 100, High: 101, Low: 95, Close: 100}}
	}
	event := Rebalance{FillTime: day, RegimeOn: true, Targets: []Rank{
		{Symbol: "BBBUSDT", Rank: 2, StopDistance: 10},
		{Symbol: "AAAUSDT", Rank: 1, StopDistance: 10},
	}}
	limits := Limits{InitialCapital: 10_000, RiskPerTradePercent: 1, MaxPositionPercent: 20, MaxPositions: 2, MaxAggregateRiskPct: 1.5}
	result, err := (Engine{Limits: limits}).Run(bars, []Rebalance{event}, day, day.Add(24*time.Hour))
	require.NoError(t, err)
	require.Len(t, result.Decisions, 2)
	require.Equal(t, protocolv2.Symbol("AAAUSDT"), result.Decisions[0].Symbol)
	require.True(t, result.Decisions[0].Accepted)
	require.Equal(t, "aggregate_risk_limit", result.Decisions[1].Reason)
	require.GreaterOrEqual(t, result.FinalCash, 0.0)
}

func TestEngineCarriesLastPriceAcrossMissingValuationDay(t *testing.T) {
	day := time.Date(2025, 1, 6, 0, 0, 0, 0, time.UTC)
	bars := map[protocolv2.Symbol][]DailyBar{
		"AAAUSDT": {
			{Time: day, Open: 100, High: 101, Low: 95, Close: 100},
			{Time: day.Add(48 * time.Hour), Open: 100, High: 101, Low: 95, Close: 100},
		},
		"BTCUSDT": {
			{Time: day, Open: 100, High: 101, Low: 99, Close: 100},
			{Time: day.Add(24 * time.Hour), Open: 100, High: 101, Low: 99, Close: 100},
			{Time: day.Add(48 * time.Hour), Open: 100, High: 101, Low: 99, Close: 100},
		},
	}
	event := Rebalance{FillTime: day, RegimeOn: true, Targets: []Rank{{Symbol: "AAAUSDT", Rank: 1, StopDistance: 10}}}
	limits := Limits{InitialCapital: 10_000, RiskPerTradePercent: 1, MaxPositionPercent: 20, MaxPositions: 1, MaxAggregateRiskPct: 5}
	result, err := (Engine{Limits: limits}).Run(bars, []Rebalance{event}, day, day.Add(72*time.Hour))
	require.NoError(t, err)
	require.Len(t, result.Equity, 3)
	require.Equal(t, 10_000.0, result.Equity[1].Equity)
}

func TestSignalArtifactChecksumIsImmutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "signals.json")
	require.NoError(t, os.WriteFile(path, []byte("frozen"), 0o644))
	ref := SignalArtifactRef{Path: path, SHA256: fileHash(t, path)}
	require.NoError(t, VerifySignalArtifacts([]SignalArtifactRef{ref}))
	require.NoError(t, os.WriteFile(path, []byte("changed"), 0o644))
	require.ErrorContains(t, VerifySignalArtifacts([]SignalArtifactRef{ref}), "checksum mismatch")
}

func TestDecisionGatesAndDiagnosticStatus(t *testing.T) {
	gates := Gates{MinNetReturn: 0, MaxDrawdown: .25, MinExcessVsBTC: -.05, MinExcessVsEqualWeight: -.05, MaxContribution: .4, RequireStressPositive: true}
	base := Metrics{NetReturn: .10, MaxDrawdown: .12, MaxProfitContributionPercent: .3}
	stress := Metrics{NetReturn: .03}
	benchmarks := Benchmarks{BTC: Metrics{NetReturn: .12}, EqualWeight: Metrics{NetReturn: .08}}
	require.Equal(t, "observe", EvaluateDecision(base, stress, benchmarks, gates, true).Status)
	require.Equal(t, "portfolio-pass", EvaluateDecision(base, stress, benchmarks, gates, false).Status)
	base.MaxDrawdown = .30
	decision := EvaluateDecision(base, stress, benchmarks, gates, false)
	require.Equal(t, "reject", decision.Status)
	require.Contains(t, decision.FailedGates, "max_drawdown")
}

func TestWorkflowVerifiesInputsAndWritesReproducibleReport(t *testing.T) {
	dir := t.TempDir()
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	symbols := []protocolv2.Symbol{"BTCUSDT", "AAAUSDT"}
	var snapshots []manifest.SymbolSnapshot
	for i, symbol := range symbols {
		path := filepath.Join(dir, string(symbol)+".csv")
		writeDailyCSV(t, path, start, 24, 100+float64(i)*10)
		snapshots = append(snapshots, manifest.SymbolSnapshot{Symbol: symbol, CandleSHA256: fileHash(t, path)})
	}
	m := Manifest{
		SchemaVersion:          ManifestSchemaVersion,
		ImplementationRevision: "test-revision",
		SourceExperiment:       "source-experiment",
		SourceManifestHash:     protocolv2.SHA256Hex(repeat("a", 64)),
		Diagnostic:             true,
		Universe:               manifest.UniverseSnapshot{Exchange: "binance", Spot: true, Symbols: snapshots},
		Range:                  protocolv2.TimeRange{Start: start.Add(10 * 24 * time.Hour), End: start.Add(24 * 24 * time.Hour)},
		BaseCosts:              CostProfile{CommissionBPS: 10, SlippageBPS: 5},
		StressCosts:            CostProfile{CommissionBPS: 10, SlippageBPS: 15},
		Limits:                 Limits{InitialCapital: 10_000, RiskPerTradePercent: 1, MaxPositionPercent: 20, MaxPositions: 2, MaxAggregateRiskPct: 5},
		RelativeStrength:       RelativeStrengthConfig{ReturnLookbackDays: 3, VolatilityDays: 3, ATRDays: 3, StopATR: 2, TopK: 1, ExitRank: 2, BTCEMADays: 3, MinPositiveBreadth: 0, RebalanceWeekday: time.Monday, RegimeMode: RegimeModeNone, EntryMode: EntryModeWeeklyOpen, PullbackEMADays: 3, MaxEntryDistanceATR: .5},
		Gates:                  Gates{MinNetReturn: -1, MaxDrawdown: 1, MinExcessVsBTC: -1, MinExcessVsEqualWeight: -1, MaxContribution: 1},
	}
	require.NoError(t, m.freeze())
	output := filepath.Join(dir, "report.json")
	report, err := Run(context.Background(), m, dir, output)
	require.NoError(t, err)
	require.FileExists(t, output)
	require.Equal(t, m.ID, report.ExperimentID)
	require.NotEmpty(t, report.Rebalances)
	require.Equal(t, "rs-90d-vol30-top5-none-weekly-open", report.Candidate)
	require.Equal(t, "observe", report.Decision.Status)
	_, err = Run(context.Background(), m, dir, output)
	require.NoError(t, err, "identical reruns must reuse the immutable artifact")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "AAAUSDT.csv"), []byte("changed"), 0o644))
	_, err = Run(context.Background(), m, dir, filepath.Join(dir, "other.json"))
	require.ErrorContains(t, err, "checksum mismatch")
}

func syntheticBars(start time.Time, count int, price, drift float64) []DailyBar {
	out := make([]DailyBar, 0, count)
	for i := 0; i < count; i++ {
		change := drift
		if i%3 == 0 {
			change -= .001
		}
		open := price
		price *= 1 + change
		out = append(out, DailyBar{Time: start.Add(time.Duration(i) * 24 * time.Hour), Open: open, High: math.Max(open, price) * 1.01, Low: minFloat(open, price) * .99, Close: price})
	}
	return out
}

func writeDailyCSV(t *testing.T, path string, start time.Time, days int, initial float64) {
	t.Helper()
	content := "open_time,open,high,low,close,volume\n"
	price := initial
	for i := 0; i < days; i++ {
		open := price
		price *= 1 + .01 + float64(i%3)*.001
		content += fmt.Sprintf("%s,%.8f,%.8f,%.8f,%.8f,100\n", start.Add(time.Duration(i)*24*time.Hour).Format("2006-01-02 15:04:05"), open, price*1.01, open*.99, price)
	}
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func fileHash(t *testing.T, path string) protocolv2.SHA256Hex {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	return protocolv2.SHA256Hex(hex.EncodeToString(sum[:]))
}

func repeat(value string, count int) string {
	out := ""
	for i := 0; i < count; i++ {
		out += value
	}
	return out
}

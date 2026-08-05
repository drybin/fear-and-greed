package orchestration_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/candidates"
	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/orchestration"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func TestInProcessRunnerAndPreflight(t *testing.T) {
	dir := t.TempDir()
	candleDir := filepath.Join(dir, "candles")
	require.NoError(t, os.MkdirAll(candleDir, 0o755))

	start := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	var b strings.Builder
	b.WriteString("open_time,open,high,low,close,volume\n")
	price := 100.0
	for i := 0; i < 24*40; i++ {
		ts := start.Add(time.Duration(i) * time.Hour)
		if i%7 == 0 {
			price++
		}
		fmt.Fprintf(&b, "%s,%.2f,%.2f,%.2f,%.2f,1\n",
			ts.Format("2006-01-02 15:04:05"), price, price+1, price-1, price)
	}
	candlePath := filepath.Join(candleDir, "BTCUSDT.csv")
	require.NoError(t, os.WriteFile(candlePath, []byte(b.String()), 0o644))
	sum := sha256.Sum256([]byte(b.String()))
	candleSHA := protocolv2.SHA256Hex(hex.EncodeToString(sum[:]))

	m := validInProcessManifest(candleSHA)
	require.NoError(t, m.Freeze())
	encoded, err := m.MarshalCanonical()
	require.NoError(t, err)
	m, err = manifest.Decode(encoded)
	require.NoError(t, err)

	store := orchestration.DirCandleStore{Dir: candleDir}
	dataHash, err := orchestration.PreflightDevelopment(m, dir, store)
	require.NoError(t, err)
	require.NotEmpty(t, dataHash)
	_, err = os.Stat(filepath.Join(protocolv2.ExperimentRoot(dir, m.ID), "reports", "eligibility.json"))
	require.NoError(t, err)

	runner, err := orchestration.NewInProcessRunner(m, store)
	require.NoError(t, err)

	rang, err := protocolv2.NewTimeRange(
		time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2022, 1, 20, 0, 0, 0, 0, time.UTC),
	)
	require.NoError(t, err)
	artifact, err := runner.Run(context.Background(), orchestration.Unit{
		Strategy:  protocolv2.StrategyRef{Code: "breakout-retest-long-v2", Version: "v2.0.0"},
		Candidate: "default",
		Fold:      "fold-0",
		Cost:      "base",
		Range:     rang,
	})
	require.NoError(t, err)
	require.NotEmpty(t, artifact)

	controlArtifact, err := runner.Run(context.Background(), orchestration.Unit{
		Strategy:  protocolv2.StrategyRef{Code: "cash-control", Version: "v1"},
		Candidate: "control",
		Fold:      "fold-0-test",
		Cost:      "base",
		Control:   "cash-control",
		Range:     rang,
	})
	require.NoError(t, err)
	require.NotEmpty(t, controlArtifact)
}

func TestInProcessRunnerRejectsManifestAdapterDrift(t *testing.T) {
	mutations := map[string]func(*manifest.Strategy){
		"version":   func(strategy *manifest.Strategy) { strategy.Ref.Version = "unexpected" },
		"timeframe": func(strategy *manifest.Strategy) { strategy.Timeframe = "4h" },
		"warmup":    func(strategy *manifest.Strategy) { strategy.WarmupBars++ },
		"grid": func(strategy *manifest.Strategy) {
			strategy.Grid[0].Values = map[string]any{"unexpected": true}
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			m := validInProcessManifest(protocolv2.SHA256Hex(strings.Repeat("a", 64)))
			mutate(&m.Strategies[0])
			require.NoError(t, m.Freeze())
			_, err := orchestration.NewInProcessRunner(m, orchestration.DirCandleStore{Dir: t.TempDir()})
			require.ErrorContains(t, err, "does not match")
		})
	}
}

func validInProcessManifest(candleSHA protocolv2.SHA256Hex) manifest.Manifest {
	train, _ := protocolv2.NewTimeRange(
		time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2022, 10, 1, 0, 0, 0, 0, time.UTC),
	)
	test, _ := protocolv2.NewTimeRange(
		time.Date(2022, 10, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
	)
	holdout, _ := protocolv2.NewTimeRange(
		time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC),
	)
	strategies := make([]manifest.Strategy, 0, 4)
	for _, adapter := range candidates.Core() {
		metadata := adapter.Metadata()
		grid := make([]manifest.ParameterCandidate, 0, len(adapter.Grid()))
		for _, candidate := range adapter.Grid() {
			grid = append(grid, manifest.ParameterCandidate{ID: candidate.ID, Values: candidate.Values})
		}
		strategies = append(strategies, manifest.Strategy{Ref: metadata.Ref, Timeframe: metadata.Timeframe, WarmupBars: metadata.WarmupBars, Grid: grid, DefaultParams: map[string]any{}})
	}
	return manifest.Manifest{
		SchemaVersion: protocolv2.ManifestSchemaVersion, ProtocolVersion: manifest.ProtocolVersion,
		Cutoff: time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC),
		Source: manifest.SourceRevision{GitRevision: "test"}, Seed: 1,
		Universe: manifest.UniverseSnapshot{
			Name: "fixture", Provenance: protocolv2.UniverseFrozenCurrentCohort,
			Exchange: "binance", Spot: true, QuoteAsset: "USDT",
			Symbols: []manifest.SymbolSnapshot{{Symbol: "BTCUSDT", CandleSHA256: candleSHA}},
		},
		Strategies: strategies,
		Schedule:   manifest.Schedule{Train: train, Test: test, FoldStep: 90 * 24 * time.Hour, LockedHoldout: holdout},
		Execution:  manifest.ExecutionProfile{ID: "base", CommissionBPS: 10, SlippageBPS: 5, GapPolicy: manifest.GapFillNextAvailable, IntrabarPolicy: manifest.IntrabarStopFirst},
		Risk:       manifest.StandaloneRisk{SizingProfile: "standalone", InitialEquity: 10000, RiskPerTradePercent: 1, MaxNotionalPercent: 20},
		Gates:      manifest.Gates{MinAggregateTrades: 1, MinEligibleSymbols: 1, MinDevelopmentFolds: 1, MinPositiveFoldFraction: 0.1, MinProfitFactor: 1, MaxMedianDrawdownPercent: 100, MaxContributionPercent: 100},
	}
}

package manifest_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func TestManifestRequiredFieldsAndInvalidCombinations(t *testing.T) {
	m := validManifest(t)
	require.NoError(t, m.Validate())

	m.Universe.Spot = false
	require.ErrorContains(t, m.Validate(), "spot")

	m = validManifest(t)
	m.Schedule.Test.Start = m.Schedule.Train.End.Add(-time.Hour)
	require.ErrorContains(t, m.Validate(), "non-overlapping")

	m = validManifest(t)
	m.Strategies[0].Grid = append(m.Strategies[0].Grid, makeCandidates(30)...)
	require.ErrorContains(t, m.Validate(), "1 to 30")

	m = validManifest(t)
	m.Execution.GapPolicy = "ideal-fill"
	require.ErrorContains(t, m.Validate(), "gap policy")
}

func TestDecodeRejectsUnknownFieldsAndStaleIdentity(t *testing.T) {
	m := validManifest(t)
	require.NoError(t, m.Freeze())
	data, err := m.MarshalCanonical()
	require.NoError(t, err)

	_, err = manifest.Decode(append(data[:len(data)-1], []byte(`,"unexpected":true}`)...))
	require.ErrorContains(t, err, "unknown field")

	m.Hash = protocolv2.SHA256Hex("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	data, err = m.MarshalCanonical()
	require.ErrorContains(t, err, "identity")
	require.Nil(t, data)
}

func TestHashIsStableAndChangesWithResearchInput(t *testing.T) {
	a := validManifest(t)
	b := validManifest(t)
	b.Universe.Symbols[0], b.Universe.Symbols[1] = b.Universe.Symbols[1], b.Universe.Symbols[0]
	aID, aHash, err := a.Identity()
	require.NoError(t, err)
	bID, bHash, err := b.Identity()
	require.NoError(t, err)
	require.Equal(t, aID, bID)
	require.Equal(t, aHash, bHash)

	b.Seed++
	bID, bHash, err = b.Identity()
	require.NoError(t, err)
	require.NotEqual(t, aID, bID)
	require.NotEqual(t, aHash, bHash)
}

func TestWriteFileRefusesChangedIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifests", "manifest.json")
	m := validManifest(t)
	require.NoError(t, m.Freeze())
	require.NoError(t, manifest.WriteFile(path, m))
	require.NoError(t, manifest.WriteFile(path, m))

	m.Seed++
	require.NoError(t, m.Freeze())
	require.ErrorContains(t, manifest.WriteFile(path, m), "refusing to overwrite")
	_, err := os.Stat(path)
	require.NoError(t, err)
}

func makeCandidates(n int) []manifest.ParameterCandidate {
	out := make([]manifest.ParameterCandidate, n)
	for i := range out {
		out[i] = manifest.ParameterCandidate{
			ID:     protocolv2.ParameterCandidateID("candidate-" + string(rune('a'+i))),
			Values: map[string]any{"length": i + 1},
		}
	}
	return out
}

func validManifest(t *testing.T) manifest.Manifest {
	t.Helper()
	start := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	rng := func(from, to time.Time) protocolv2.TimeRange {
		r, err := protocolv2.NewTimeRange(from, to)
		require.NoError(t, err)
		return r
	}
	return manifest.Manifest{
		SchemaVersion:   protocolv2.ManifestSchemaVersion,
		ProtocolVersion: manifest.ProtocolVersion,
		Cutoff:          start.AddDate(1, 6, 0),
		Source:          manifest.SourceRevision{GitRevision: "0123456789abcdef"},
		Seed:            42,
		Universe: manifest.UniverseSnapshot{
			Name:       "top-50",
			Provenance: protocolv2.UniverseFrozenCurrentCohort,
			Exchange:   "binance",
			Spot:       true,
			QuoteAsset: "USDT",
			Symbols: []manifest.SymbolSnapshot{
				{Symbol: "ETHUSDT", CandleSHA256: protocolv2.SHA256Hex("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")},
				{Symbol: "BTCUSDT", CandleSHA256: protocolv2.SHA256Hex("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")},
			},
			Exclusions: []protocolv2.Symbol{"USDTUSDT"},
		},
		Strategies: []manifest.Strategy{{
			Ref:       protocolv2.StrategyRef{Code: "nr7-trend-breakout-v1", Version: "v1"},
			Timeframe: "1h", WarmupBars: 200, DefaultParams: map[string]any{"length": 7},
			Grid: []manifest.ParameterCandidate{
				{ID: "slow", Values: map[string]any{"length": 14}},
				{ID: "fast", Values: map[string]any{"length": 7}},
			},
		}},
		Schedule: manifest.Schedule{
			Train:         rng(start, start.AddDate(0, 9, 0)),
			Test:          rng(start.AddDate(0, 9, 0), start.AddDate(1, 0, 0)),
			FoldStep:      3 * 30 * 24 * time.Hour,
			LockedHoldout: rng(start.AddDate(1, 0, 0), start.AddDate(1, 3, 0)),
		},
		Execution: manifest.ExecutionProfile{ID: "base", CommissionBPS: 10, SlippageBPS: 5, GapPolicy: manifest.GapReject, IntrabarPolicy: manifest.IntrabarStopFirst},
		Risk:      manifest.StandaloneRisk{SizingProfile: "one-percent", InitialEquity: 10000, RiskPerTradePercent: 1, MaxNotionalPercent: 20},
		Gates:     manifest.Gates{MinAggregateTrades: 100, MinEligibleSymbols: 20, MinDevelopmentFolds: 3, MinPositiveFoldFraction: .6, MinProfitFactor: 1.15, RequirePositiveExpectancy: true, MaxMedianDrawdownPercent: 20, MaxContributionPercent: 25, RequireStressPositive: true, RequireParameterStability: true, RequireHoldoutConsistency: true},
	}
}

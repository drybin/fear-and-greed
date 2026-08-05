package validation

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func utc(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func TestGenerateFoldsReservesHoldout(t *testing.T) {
	schedule := manifest.Schedule{
		Train:         protocolv2.TimeRange{Start: utc(2020, 1, 1), End: utc(2020, 10, 1)},
		Test:          protocolv2.TimeRange{Start: utc(2020, 10, 1), End: utc(2021, 1, 1)},
		FoldStep:      90 * 24 * time.Hour,
		LockedHoldout: protocolv2.TimeRange{Start: utc(2021, 7, 1), End: utc(2021, 10, 1)},
	}
	folds, err := GenerateFolds(schedule, 30*24*time.Hour)
	require.NoError(t, err)
	require.Len(t, folds, 3)
	for _, fold := range folds {
		require.False(t, fold.Train.Overlaps(fold.Test))
		require.False(t, fold.Test.Overlaps(schedule.LockedHoldout))
		require.False(t, fold.Warmup.Overlaps(fold.Train))
		require.True(t, fold.Warmup.End.Equal(fold.Train.Start))
	}
	require.Equal(t, protocolv2.ExperimentPromotable, ClassifyExperiment(folds))
}

func TestScoreCandidatesCompleteCohortAndTieBreak(t *testing.T) {
	symbols := []protocolv2.Symbol{"BTCUSDT", "ETHUSDT"}
	candidates := []CandidateEvidence{
		{Candidate: "z", Symbols: []SymbolEvidence{{Symbol: "BTCUSDT", NetPnL: 10, MaxDrawdownPercent: 2, CompletedTrades: 60}, {Symbol: "ETHUSDT", NetPnL: 10, MaxDrawdownPercent: 2, CompletedTrades: 60}}},
		{Candidate: "a", Symbols: []SymbolEvidence{{Symbol: "BTCUSDT", NetPnL: 10, MaxDrawdownPercent: 2, CompletedTrades: 60}, {Symbol: "ETHUSDT", NetPnL: 10, MaxDrawdownPercent: 2, CompletedTrades: 60}}},
	}
	scores, err := ScoreCandidates(symbols, candidates, ScoreConfig{MinAggregateTrades: 100})
	require.NoError(t, err)
	require.Equal(t, protocolv2.ParameterCandidateID("a"), scores[0].Candidate)
	_, err = ScoreCandidates(symbols, []CandidateEvidence{{Candidate: "only", Symbols: candidates[0].Symbols[:1]}}, ScoreConfig{})
	require.Error(t, err)
}

func TestPoisonedTestCannotChangeSelection(t *testing.T) {
	symbols := []protocolv2.Symbol{"BTCUSDT", "ETHUSDT"}
	training := []CandidateEvidence{
		{Candidate: "safe", Symbols: []SymbolEvidence{{Symbol: "BTCUSDT", NetPnL: 4, CompletedTrades: 50}, {Symbol: "ETHUSDT", NetPnL: 4, CompletedTrades: 50}}},
		{Candidate: "risky", Symbols: []SymbolEvidence{{Symbol: "BTCUSDT", NetPnL: 3, CompletedTrades: 50}, {Symbol: "ETHUSDT", NetPnL: 3, CompletedTrades: 50}}},
	}
	scores, err := ScoreCandidates(symbols, training, ScoreConfig{MinAggregateTrades: 100})
	require.NoError(t, err)
	selection, err := Select(protocolv2.StrategyRef{Code: "test", Version: "v1"}, "fold-001", scores)
	require.NoError(t, err)
	assignments, err := ApplySelection(selection, symbols)
	require.NoError(t, err)
	// These intentionally absurd test observations never enter ScoreCandidates.
	poisonedTest := map[protocolv2.Symbol]float64{"BTCUSDT": -1e99, "ETHUSDT": 1e99}
	require.NotEmpty(t, poisonedTest)
	require.Equal(t, protocolv2.ParameterCandidateID("safe"), selection.Candidate)
	require.Equal(t, selection.Candidate, assignments["BTCUSDT"])
	require.Equal(t, selection.Candidate, assignments["ETHUSDT"])
}

func TestPoisonedHoldoutLeavesDevelopmentArtifactByteIdentical(t *testing.T) {
	gates := testGates()
	development := GateInput{AggregateTrades: 120, EligibleSymbols: 21, DevelopmentFolds: 3, PositiveFoldFraction: .7, ProfitFactor: 1.2, Expectancy: 1, MedianDrawdownPercent: 10, MaxContributionPct: 20, StressPositive: true, ParameterStable: true, NeighborsRobust: true, HoldoutConsistent: true}
	before, err := EvaluateGates(gates, development)
	require.NoError(t, err)
	beforeBytes, err := json.Marshal(before)
	require.NoError(t, err)
	poisonedHoldoutPnL := -1e99 // Holdout observations are not accepted by development APIs.
	require.Less(t, poisonedHoldoutPnL, 0.0)
	after, err := EvaluateGates(gates, development)
	require.NoError(t, err)
	afterBytes, err := json.Marshal(after)
	require.NoError(t, err)
	require.Equal(t, beforeBytes, afterBytes)
}

func TestGatesEveryFinalDecisionAndGate(t *testing.T) {
	gates := testGates()
	base := GateInput{AggregateTrades: 120, EligibleSymbols: 21, DevelopmentFolds: 3, PositiveFoldFraction: .7, ProfitFactor: 1.2, Expectancy: 1, MedianDrawdownPercent: 10, MaxContributionPct: 20, StressPositive: true, ParameterStable: true, NeighborsRobust: true, HoldoutConsistent: true}
	decision, err := EvaluateGates(gates, base)
	require.NoError(t, err)
	require.Equal(t, protocolv2.DecisionResearchPass, decision.Status)
	require.Len(t, decision.Gates, 12)

	for name, mutate := range map[string]func(*GateInput){
		"trades":         func(v *GateInput) { v.AggregateTrades = 99 },
		"symbols":        func(v *GateInput) { v.EligibleSymbols = 19 },
		"folds":          func(v *GateInput) { v.DevelopmentFolds = 2 },
		"positive folds": func(v *GateInput) { v.PositiveFoldFraction = .5 },
		"profit factor":  func(v *GateInput) { v.ProfitFactor = 1.1 },
		"expectancy":     func(v *GateInput) { v.Expectancy = 0 },
		"drawdown":       func(v *GateInput) { v.MedianDrawdownPercent = 21 },
		"concentration":  func(v *GateInput) { v.MaxContributionPct = 26 },
		"stress":         func(v *GateInput) { v.StressPositive = false },
		"stability":      func(v *GateInput) { v.ParameterStable = false },
		"neighbors":      func(v *GateInput) { v.NeighborsRobust = false },
		"holdout":        func(v *GateInput) { v.HoldoutConsistent = false },
	} {
		t.Run(name, func(t *testing.T) {
			input := base
			mutate(&input)
			got, err := EvaluateGates(gates, input)
			require.NoError(t, err)
			switch name {
			case "folds":
				require.Equal(t, protocolv2.DecisionExploratory, got.Status)
			case "expectancy", "drawdown", "concentration", "stress":
				require.Equal(t, protocolv2.DecisionReject, got.Status)
			default:
				require.Equal(t, protocolv2.DecisionObserve, got.Status)
			}
		})
	}
}

func TestSensitivityAndParameterStability(t *testing.T) {
	scores := []CandidateScore{{Candidate: "a", Score: 100}, {Candidate: "b", Score: 70}, {Candidate: "c", Score: 95}}
	sensitivity, err := NeighboringSensitivity([]protocolv2.ParameterCandidateID{"a", "b", "c"}, scores, "a", 20)
	require.NoError(t, err)
	require.Len(t, sensitivity, 1)
	require.True(t, sensitivity[0].MaterialCollapse)
	stability, err := AssessParameterStability([]Selection{{Candidate: "a"}, {Candidate: "a"}, {Candidate: "b"}}, .6)
	require.NoError(t, err)
	require.True(t, stability.Stable)
	require.Equal(t, protocolv2.ParameterCandidateID("a"), stability.MostFrequent)
}

func testGates() manifest.Gates {
	return manifest.Gates{
		MinAggregateTrades: 100, MinEligibleSymbols: 20, MinDevelopmentFolds: 3,
		MinPositiveFoldFraction: .6, MinProfitFactor: 1.15, RequirePositiveExpectancy: true,
		MaxMedianDrawdownPercent: 20, MaxContributionPercent: 25, RequireStressPositive: true,
		RequireParameterStability: true, RequireHoldoutConsistency: true,
	}
}

// Package validation implements the chronological, train-only parts of the
// protocol-v2 research process. It deliberately accepts metric values as
// small local inputs so that it is independent of report construction.
package validation

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

const (
	minimumTrain = 9 * 30 * 24 * time.Hour
	minimumTest  = 3 * 30 * 24 * time.Hour
	minimumStep  = 3 * 30 * 24 * time.Hour
)

// Fold is one chronological development evaluation. Warmup ends at the train
// start and never contributes to either selection or test evidence.
type Fold struct {
	ID     protocolv2.FoldID    `json:"id"`
	Warmup protocolv2.TimeRange `json:"warmup"`
	Train  protocolv2.TimeRange `json:"train"`
	Test   protocolv2.TimeRange `json:"test"`
}

// GenerateFolds reserves LockedHoldout before producing rolling development
// folds. Schedule.Train and Schedule.Test define the first fold. A later fold
// is shifted by FoldStep, with no fold allowed to read the holdout.
func GenerateFolds(schedule manifest.Schedule, warmup time.Duration) ([]Fold, error) {
	if err := validateSchedule(schedule); err != nil {
		return nil, err
	}
	if warmup < 0 {
		return nil, fmt.Errorf("validation: warmup cannot be negative")
	}

	var folds []Fold
	for n, train, test := 1, schedule.Train, schedule.Test; !test.End.After(schedule.LockedHoldout.Start); n, train, test = n+1, shift(train, schedule.FoldStep), shift(test, schedule.FoldStep) {
		warmupRange, err := protocolv2.NewTimeRange(train.Start.Add(-warmup), train.Start)
		if err != nil {
			return nil, err
		}
		folds = append(folds, Fold{
			ID:     protocolv2.FoldID(fmt.Sprintf("fold-%03d", n)),
			Warmup: warmupRange,
			Train:  train,
			Test:   test,
		})
	}
	if len(folds) == 0 {
		return nil, fmt.Errorf("validation: no development folds before locked holdout")
	}
	return folds, nil
}

func validateSchedule(s manifest.Schedule) error {
	for _, item := range []struct {
		name string
		r    protocolv2.TimeRange
	}{{"train", s.Train}, {"test", s.Test}, {"locked holdout", s.LockedHoldout}} {
		if err := item.r.Validate(); err != nil {
			return fmt.Errorf("validation: %s: %w", item.name, err)
		}
	}
	if s.Train.Duration() < minimumTrain || s.Test.Duration() < minimumTest || s.FoldStep < minimumStep {
		return fmt.Errorf("validation: requires at least 9-month train, 3-month test, and 3-month step")
	}
	if s.FoldStep <= 0 || !s.Train.End.Equal(s.Test.Start) || s.Test.End.After(s.LockedHoldout.Start) {
		return fmt.Errorf("validation: initial train, test, and locked holdout must be chronological and non-overlapping")
	}
	return nil
}

func shift(r protocolv2.TimeRange, by time.Duration) protocolv2.TimeRange {
	return protocolv2.TimeRange{Start: r.Start.Add(by), End: r.End.Add(by)}
}

// ClassifyExperiment makes the three-development-fold rule explicit before
// decisions are evaluated.
func ClassifyExperiment(folds []Fold) protocolv2.ExperimentClass {
	if len(folds) < 3 {
		return protocolv2.ExperimentExploratory
	}
	return protocolv2.ExperimentPromotable
}

// SymbolEvidence is the minimum train-only metric input used by selection.
// NetPnL is deliberately mark-to-market net PnL, not realized cash.
type SymbolEvidence struct {
	Symbol             protocolv2.Symbol `json:"symbol"`
	NetPnL             float64           `json:"net_pnl"`
	MaxDrawdownPercent float64           `json:"max_drawdown_percent"`
	CompletedTrades    int               `json:"completed_trades"`
}

// CandidateEvidence must contain one result for every eligible training
// symbol. Test and holdout observations intentionally have no place here.
type CandidateEvidence struct {
	Candidate protocolv2.ParameterCandidateID `json:"candidate"`
	Symbols   []SymbolEvidence                `json:"symbols"`
}

// ScoreConfig freezes penalty weights for global candidate selection.
type ScoreConfig struct {
	DrawdownPenalty      float64 `json:"drawdown_penalty"`
	LowSamplePenalty     float64 `json:"low_sample_penalty"`
	MinAggregateTrades   int     `json:"min_aggregate_trades"`
	ConcentrationPenalty float64 `json:"concentration_penalty"`
}

// CandidateScore preserves every score component for deterministic reporting.
type CandidateScore struct {
	Candidate            protocolv2.ParameterCandidateID `json:"candidate"`
	MedianSymbolEvidence float64                         `json:"median_symbol_evidence"`
	DrawdownPenalty      float64                         `json:"drawdown_penalty"`
	SamplePenalty        float64                         `json:"sample_penalty"`
	ConcentrationPenalty float64                         `json:"concentration_penalty"`
	AggregateTrades      int                             `json:"aggregate_trades"`
	PositiveContribution float64                         `json:"positive_contribution"`
	MaxContributionPct   float64                         `json:"max_contribution_percent"`
	Score                float64                         `json:"score"`
}

// ScoreCandidates evaluates each candidate over exactly eligibleSymbols.
// Results are returned in deterministic winner-first order (score, then ID).
func ScoreCandidates(eligibleSymbols []protocolv2.Symbol, candidates []CandidateEvidence, config ScoreConfig) ([]CandidateScore, error) {
	if len(eligibleSymbols) == 0 || len(candidates) == 0 || len(candidates) > 30 {
		return nil, fmt.Errorf("validation: require 1-30 candidates and an eligible training cohort")
	}
	if config.DrawdownPenalty < 0 || config.LowSamplePenalty < 0 || config.ConcentrationPenalty < 0 || config.MinAggregateTrades < 0 {
		return nil, fmt.Errorf("validation: invalid score config")
	}
	expected := uniqueSymbols(eligibleSymbols)
	if len(expected) != len(eligibleSymbols) {
		return nil, fmt.Errorf("validation: duplicate eligible training symbol")
	}
	seenCandidates := make(map[protocolv2.ParameterCandidateID]bool, len(candidates))
	scores := make([]CandidateScore, 0, len(candidates))
	for _, candidate := range candidates {
		if err := candidate.Candidate.Validate(); err != nil || seenCandidates[candidate.Candidate] {
			return nil, fmt.Errorf("validation: candidate IDs must be valid and unique")
		}
		seenCandidates[candidate.Candidate] = true
		if len(candidate.Symbols) != len(expected) {
			return nil, fmt.Errorf("validation: candidate %q does not cover complete training cohort", candidate.Candidate)
		}
		bySymbol := make(map[protocolv2.Symbol]SymbolEvidence, len(candidate.Symbols))
		for _, evidence := range candidate.Symbols {
			if _, ok := expected[evidence.Symbol]; !ok || !finite(evidence.NetPnL) || !finite(evidence.MaxDrawdownPercent) || evidence.MaxDrawdownPercent < 0 || evidence.CompletedTrades < 0 {
				return nil, fmt.Errorf("validation: invalid training evidence for %q", candidate.Candidate)
			}
			if _, duplicate := bySymbol[evidence.Symbol]; duplicate {
				return nil, fmt.Errorf("validation: duplicate training evidence for %q", evidence.Symbol)
			}
			bySymbol[evidence.Symbol] = evidence
		}
		evidence, drawdowns := make([]float64, 0, len(expected)), make([]float64, 0, len(expected))
		totalTrades, positive, largest := 0, 0.0, 0.0
		for symbol := range expected {
			item, ok := bySymbol[symbol]
			if !ok {
				return nil, fmt.Errorf("validation: candidate %q missing training symbol %q", candidate.Candidate, symbol)
			}
			evidence, drawdowns = append(evidence, item.NetPnL), append(drawdowns, item.MaxDrawdownPercent)
			totalTrades += item.CompletedTrades
			if item.NetPnL > 0 {
				positive += item.NetPnL
				if item.NetPnL > largest {
					largest = item.NetPnL
				}
			}
		}
		contribution := 0.0
		if positive > 0 {
			contribution = largest / positive * 100
		}
		sampleGap := 0
		if totalTrades < config.MinAggregateTrades {
			sampleGap = config.MinAggregateTrades - totalTrades
		}
		score := median(evidence) - config.DrawdownPenalty*median(drawdowns) - config.LowSamplePenalty*float64(sampleGap) - config.ConcentrationPenalty*contribution
		scores = append(scores, CandidateScore{Candidate: candidate.Candidate, MedianSymbolEvidence: median(evidence), DrawdownPenalty: config.DrawdownPenalty * median(drawdowns), SamplePenalty: config.LowSamplePenalty * float64(sampleGap), ConcentrationPenalty: config.ConcentrationPenalty * contribution, AggregateTrades: totalTrades, PositiveContribution: positive, MaxContributionPct: contribution, Score: score})
	}
	sort.Slice(scores, func(i, j int) bool {
		if scores[i].Score == scores[j].Score {
			return scores[i].Candidate < scores[j].Candidate
		}
		return scores[i].Score > scores[j].Score
	})
	return scores, nil
}

// Selection records the only candidate that may be used for one strategy fold.
type Selection struct {
	Strategy  protocolv2.StrategyRef          `json:"strategy"`
	Fold      protocolv2.FoldID               `json:"fold"`
	Candidate protocolv2.ParameterCandidateID `json:"candidate"`
	Score     CandidateScore                  `json:"score"`
}

func Select(strategy protocolv2.StrategyRef, fold protocolv2.FoldID, scores []CandidateScore) (Selection, error) {
	if err := strategy.Validate(); err != nil || fold.Validate() != nil || len(scores) == 0 {
		return Selection{}, fmt.Errorf("validation: valid strategy, fold, and candidate scores are required")
	}
	return Selection{Strategy: strategy, Fold: fold, Candidate: scores[0].Candidate, Score: scores[0]}, nil
}

// ApplySelection binds the selected candidate, unchanged, to every test symbol.
func ApplySelection(selection Selection, testSymbols []protocolv2.Symbol) (map[protocolv2.Symbol]protocolv2.ParameterCandidateID, error) {
	if len(testSymbols) == 0 || selection.Candidate.Validate() != nil {
		return nil, fmt.Errorf("validation: selected candidate and test symbols are required")
	}
	out := make(map[protocolv2.Symbol]protocolv2.ParameterCandidateID, len(testSymbols))
	for _, symbol := range testSymbols {
		if symbol.Validate() != nil {
			return nil, fmt.Errorf("validation: invalid test symbol %q", symbol)
		}
		if _, duplicate := out[symbol]; duplicate {
			return nil, fmt.Errorf("validation: duplicate test symbol %q", symbol)
		}
		out[symbol] = selection.Candidate
	}
	return out, nil
}

type NeighborSensitivity struct {
	Selected         protocolv2.ParameterCandidateID `json:"selected"`
	Neighbor         protocolv2.ParameterCandidateID `json:"neighbor"`
	ScoreDropPercent float64                         `json:"score_drop_percent"`
	MaterialCollapse bool                            `json:"material_collapse"`
}

// NeighboringSensitivity assesses the declared grid order. maxDropPercent is
// the largest acceptable selected-to-neighbor score drop.
func NeighboringSensitivity(grid []protocolv2.ParameterCandidateID, scores []CandidateScore, selected protocolv2.ParameterCandidateID, maxDropPercent float64) ([]NeighborSensitivity, error) {
	if maxDropPercent < 0 || maxDropPercent > 100 {
		return nil, fmt.Errorf("validation: invalid maximum score drop")
	}
	index := -1
	for i, id := range grid {
		if id == selected {
			index = i
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("validation: selected candidate missing from grid")
	}
	byID := make(map[protocolv2.ParameterCandidateID]CandidateScore, len(scores))
	for _, score := range scores {
		byID[score.Candidate] = score
	}
	base, ok := byID[selected]
	if !ok {
		return nil, fmt.Errorf("validation: selected candidate score missing")
	}
	var out []NeighborSensitivity
	for _, neighborIndex := range []int{index - 1, index + 1} {
		if neighborIndex < 0 || neighborIndex >= len(grid) {
			continue
		}
		neighbor, ok := byID[grid[neighborIndex]]
		if !ok {
			return nil, fmt.Errorf("validation: neighboring candidate score missing")
		}
		drop := 0.0
		if base.Score != 0 {
			drop = (base.Score - neighbor.Score) / math.Abs(base.Score) * 100
		}
		out = append(out, NeighborSensitivity{Selected: selected, Neighbor: neighbor.Candidate, ScoreDropPercent: drop, MaterialCollapse: drop > maxDropPercent})
	}
	return out, nil
}

type ParameterStability struct {
	MostFrequent      protocolv2.ParameterCandidateID `json:"most_frequent_candidate"`
	AgreementFraction float64                         `json:"agreement_fraction"`
	Stable            bool                            `json:"stable"`
}

func AssessParameterStability(selections []Selection, minAgreement float64) (ParameterStability, error) {
	if len(selections) == 0 || minAgreement <= 0 || minAgreement > 1 {
		return ParameterStability{}, fmt.Errorf("validation: selections and agreement fraction are required")
	}
	counts := map[protocolv2.ParameterCandidateID]int{}
	for _, selection := range selections {
		counts[selection.Candidate]++
	}
	var best protocolv2.ParameterCandidateID
	for candidate, count := range counts {
		if count > counts[best] || (count == counts[best] && candidate < best) {
			best = candidate
		}
	}
	fraction := float64(counts[best]) / float64(len(selections))
	return ParameterStability{MostFrequent: best, AgreementFraction: fraction, Stable: fraction >= minAgreement}, nil
}

// GateInput is the frozen aggregate evidence used for a final gate decision.
type GateInput struct {
	AggregateTrades       int     `json:"aggregate_trades"`
	EligibleSymbols       int     `json:"eligible_symbols"`
	DevelopmentFolds      int     `json:"development_folds"`
	PositiveFoldFraction  float64 `json:"positive_fold_fraction"`
	ProfitFactor          float64 `json:"profit_factor"`
	Expectancy            float64 `json:"expectancy"`
	MedianDrawdownPercent float64 `json:"median_drawdown_percent"`
	MaxContributionPct    float64 `json:"max_contribution_percent"`
	StressPositive        bool    `json:"stress_positive"`
	ParameterStable       bool    `json:"parameter_stable"`
	NeighborsRobust       bool    `json:"neighbors_robust"`
	HoldoutConsistent     bool    `json:"holdout_consistent"`
}

type GateResult struct {
	Name        string `json:"name"`
	Threshold   any    `json:"threshold"`
	Input       any    `json:"input"`
	Passed      bool   `json:"passed"`
	Explanation string `json:"explanation"`
}

type Decision struct {
	Class       protocolv2.ExperimentClass `json:"class"`
	Status      protocolv2.DecisionStatus  `json:"status"`
	Gates       []GateResult               `json:"gates"`
	Explanation string                     `json:"explanation"`
}

// EvaluateGates applies manifest-frozen thresholds without mutating them.
func EvaluateGates(g manifest.Gates, input GateInput) (Decision, error) {
	if err := validateGates(g, input); err != nil {
		return Decision{}, err
	}
	class := protocolv2.ExperimentPromotable
	if input.DevelopmentFolds < g.MinDevelopmentFolds {
		class = protocolv2.ExperimentExploratory
	}
	gates := []GateResult{
		atLeast("aggregate_trades", g.MinAggregateTrades, input.AggregateTrades),
		atLeast("eligible_symbols", g.MinEligibleSymbols, input.EligibleSymbols),
		atLeast("development_folds", g.MinDevelopmentFolds, input.DevelopmentFolds),
		atLeastFloat("positive_fold_fraction", g.MinPositiveFoldFraction, input.PositiveFoldFraction),
		atLeastFloat("profit_factor", g.MinProfitFactor, input.ProfitFactor),
		{Name: "positive_expectancy", Threshold: g.RequirePositiveExpectancy, Input: input.Expectancy, Passed: !g.RequirePositiveExpectancy || input.Expectancy > 0, Explanation: "expectancy must be positive when required"},
		atMostFloat("median_drawdown_percent", g.MaxMedianDrawdownPercent, input.MedianDrawdownPercent),
		atMostFloat("max_contribution_percent", g.MaxContributionPercent, input.MaxContributionPct),
		{Name: "stress_positive", Threshold: g.RequireStressPositive, Input: input.StressPositive, Passed: !g.RequireStressPositive || input.StressPositive, Explanation: "stress-cost result must be positive when required"},
		{Name: "parameter_stability", Threshold: g.RequireParameterStability, Input: input.ParameterStable, Passed: !g.RequireParameterStability || input.ParameterStable, Explanation: "cross-fold parameters must be stable when required"},
		{Name: "neighbor_sensitivity", Threshold: true, Input: input.NeighborsRobust, Passed: input.NeighborsRobust, Explanation: "neighboring candidates must not materially collapse"},
		{Name: "holdout_consistency", Threshold: g.RequireHoldoutConsistency, Input: input.HoldoutConsistent, Passed: !g.RequireHoldoutConsistency || input.HoldoutConsistent, Explanation: "holdout must remain consistent when required"},
	}
	if class == protocolv2.ExperimentExploratory {
		return Decision{Class: class, Status: protocolv2.DecisionExploratory, Gates: gates, Explanation: "fewer than the required development test folds"}, nil
	}
	if input.Expectancy <= 0 || (g.RequireStressPositive && !input.StressPositive) || input.MedianDrawdownPercent > g.MaxMedianDrawdownPercent || input.MaxContributionPct > g.MaxContributionPercent {
		return Decision{Class: class, Status: protocolv2.DecisionReject, Gates: gates, Explanation: "failed expectancy, stress, or risk gate"}, nil
	}
	for _, gate := range gates {
		if !gate.Passed {
			return Decision{Class: class, Status: protocolv2.DecisionObserve, Gates: gates, Explanation: "net-positive evidence did not satisfy every research-pass gate"}, nil
		}
	}
	return Decision{Class: class, Status: protocolv2.DecisionResearchPass, Gates: gates, Explanation: "all frozen research gates passed"}, nil
}

func validateGates(g manifest.Gates, in GateInput) error {
	if g.MinAggregateTrades <= 0 || g.MinEligibleSymbols <= 0 || g.MinDevelopmentFolds <= 0 || g.MinPositiveFoldFraction <= 0 || g.MinPositiveFoldFraction > 1 || g.MinProfitFactor <= 0 || g.MaxMedianDrawdownPercent <= 0 || g.MaxMedianDrawdownPercent > 100 || g.MaxContributionPercent <= 0 || g.MaxContributionPercent > 100 {
		return fmt.Errorf("validation: invalid frozen gates")
	}
	if in.AggregateTrades < 0 || in.EligibleSymbols < 0 || in.DevelopmentFolds < 0 || !finite(in.PositiveFoldFraction) || in.PositiveFoldFraction < 0 || in.PositiveFoldFraction > 1 || !finite(in.ProfitFactor) || in.ProfitFactor < 0 || !finite(in.Expectancy) || !finite(in.MedianDrawdownPercent) || in.MedianDrawdownPercent < 0 || !finite(in.MaxContributionPct) || in.MaxContributionPct < 0 {
		return fmt.Errorf("validation: invalid gate input")
	}
	return nil
}

func atLeast(name string, threshold, input int) GateResult {
	return GateResult{Name: name, Threshold: threshold, Input: input, Passed: input >= threshold, Explanation: fmt.Sprintf("requires at least %d", threshold)}
}
func atLeastFloat(name string, threshold, input float64) GateResult {
	return GateResult{Name: name, Threshold: threshold, Input: input, Passed: input >= threshold, Explanation: fmt.Sprintf("requires at least %g", threshold)}
}
func atMostFloat(name string, threshold, input float64) GateResult {
	return GateResult{Name: name, Threshold: threshold, Input: input, Passed: input <= threshold, Explanation: fmt.Sprintf("requires no more than %g", threshold)}
}
func uniqueSymbols(symbols []protocolv2.Symbol) map[protocolv2.Symbol]struct{} {
	out := make(map[protocolv2.Symbol]struct{}, len(symbols))
	for _, symbol := range symbols {
		out[symbol] = struct{}{}
	}
	return out
}
func median(values []float64) float64 {
	sort.Float64s(values)
	mid := len(values) / 2
	if len(values)%2 == 1 {
		return values[mid]
	}
	return (values[mid-1] + values[mid]) / 2
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

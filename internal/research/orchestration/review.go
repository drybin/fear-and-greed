package orchestration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/drybin/fear-and-greed/internal/research/validation"
)

const developmentReviewSchema = "protocol-v2.development-review.v1"

// UnitEvidenceSummary is the compact decision evidence retained for one test
// or control unit. Raw trades and equity remain in the checksum-protected
// checkpoint artifact.
type UnitEvidenceSummary struct {
	Unit                  Unit    `json:"unit"`
	NetPnL                float64 `json:"net_pnl"`
	ClosedTrades          int     `json:"closed_trades"`
	EligibleSymbols       int     `json:"eligible_symbols"`
	PositiveSymbols       int     `json:"positive_symbols"`
	ProfitFactor          float64 `json:"profit_factor"`
	Expectancy            float64 `json:"expectancy"`
	MedianDrawdownPercent float64 `json:"median_drawdown_percent"`
	MaxContributionPct    float64 `json:"max_contribution_percent"`
}

type DevelopmentFoldReview struct {
	Fold      protocolv2.FoldID               `json:"fold"`
	Candidate protocolv2.ParameterCandidateID `json:"candidate"`
	Base      UnitEvidenceSummary             `json:"base"`
	Stress    UnitEvidenceSummary             `json:"stress"`
}

type DevelopmentAggregateReview struct {
	AggregateTrades       int     `json:"aggregate_trades"`
	EligibleSymbols       int     `json:"eligible_symbols"`
	DevelopmentFolds      int     `json:"development_folds"`
	PositiveFoldFraction  float64 `json:"positive_fold_fraction"`
	ProfitFactor          float64 `json:"profit_factor"`
	Expectancy            float64 `json:"expectancy"`
	MedianDrawdownPercent float64 `json:"median_drawdown_percent"`
	MaxContributionPct    float64 `json:"max_contribution_percent"`
	StressNetPnL          float64 `json:"stress_net_pnl"`
	StressPositive        bool    `json:"stress_positive"`
	ParameterStable       bool    `json:"parameter_stable"`
	NeighborsRobust       bool    `json:"neighbors_robust"`
}

type DevelopmentStrategyReview struct {
	Strategy                protocolv2.StrategyRef          `json:"strategy"`
	FrozenCandidate         protocolv2.ParameterCandidateID `json:"frozen_candidate"`
	Stability               validation.ParameterStability   `json:"stability"`
	Folds                   []DevelopmentFoldReview         `json:"folds"`
	Aggregate               DevelopmentAggregateReview      `json:"aggregate"`
	IrreversibleFailedGates []string                        `json:"irreversible_failed_gates"`
	PreHoldoutGateFlags     []string                        `json:"pre_holdout_gate_flags"`
}

type DevelopmentReview struct {
	SchemaVersion         string                      `json:"schema_version"`
	ExperimentID          protocolv2.ExperimentID     `json:"experiment_id"`
	ManifestHash          protocolv2.SHA256Hex        `json:"manifest_hash"`
	SourceHash            protocolv2.SHA256Hex        `json:"source_hash"`
	DataHash              protocolv2.SHA256Hex        `json:"data_hash"`
	DevelopmentReportHash protocolv2.SHA256Hex        `json:"development_report_hash"`
	GeneratedAt           time.Time                   `json:"generated_at"`
	Strategies            []DevelopmentStrategyReview `json:"strategies"`
	Controls              []UnitEvidenceSummary       `json:"controls"`
}

// ReviewDevelopment produces pre-holdout evidence without reading or creating
// anything under the holdout directory.
func ReviewDevelopment(outputDir string, m manifest.Manifest, sourceHash, dataHash protocolv2.SHA256Hex) (DevelopmentReview, error) {
	root := experimentRoot(outputDir, m)
	reportPath := filepath.Join(protocolv2.ReportDir(root), "development.json")
	raw, development, err := loadDevelopmentReport(reportPath, m, sourceHash, dataHash)
	if err != nil {
		return DevelopmentReview{}, err
	}
	if err := validateDevelopmentCompleteness(root, m, development); err != nil {
		return DevelopmentReview{}, err
	}
	candidates, err := deriveFinalCandidates(m, development.Selections)
	if err != nil {
		return DevelopmentReview{}, err
	}

	review := DevelopmentReview{
		SchemaVersion: developmentReviewSchema, ExperimentID: m.ID, ManifestHash: m.Hash,
		SourceHash: sourceHash, DataHash: dataHash, DevelopmentReportHash: digest(raw), GeneratedAt: time.Now().UTC(),
	}
	for _, candidate := range candidates {
		strategyReview, err := reviewStrategy(root, m, development, candidate)
		if err != nil {
			return DevelopmentReview{}, err
		}
		review.Strategies = append(review.Strategies, strategyReview)
		releaseResearchMemory()
	}
	for _, checkpoint := range development.Units {
		if checkpoint.Unit.Control == "" {
			continue
		}
		result, err := readCheckpointArtifact(root, checkpoint)
		if err != nil {
			return DevelopmentReview{}, err
		}
		review.Controls = append(review.Controls, summarizeUnitEvidence(result, m.Risk.InitialEquity))
		releaseResearchMemory()
	}
	sort.Slice(review.Controls, func(i, j int) bool { return review.Controls[i].Unit.Key() < review.Controls[j].Unit.Key() })
	if _, err := writeJSONAtomic(filepath.Join(protocolv2.ReportDir(root), "development-review.json"), review); err != nil {
		return DevelopmentReview{}, err
	}
	return review, nil
}

func loadDevelopmentReport(path string, m manifest.Manifest, sourceHash, dataHash protocolv2.SHA256Hex) ([]byte, DevelopmentReport, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, DevelopmentReport{}, fmt.Errorf("orchestration: read development report: %w", err)
	}
	var report DevelopmentReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, DevelopmentReport{}, fmt.Errorf("orchestration: invalid development report: %w", err)
	}
	if report.ExperimentID != m.ID || report.ManifestHash != m.Hash || report.SourceHash != sourceHash || report.DataHash != dataHash {
		return nil, DevelopmentReport{}, fmt.Errorf("orchestration: development report does not match frozen inputs")
	}
	return raw, report, nil
}

func reviewStrategy(root string, m manifest.Manifest, development DevelopmentReport, candidate FinalCandidate) (DevelopmentStrategyReview, error) {
	review := DevelopmentStrategyReview{Strategy: candidate.Strategy, FrozenCandidate: candidate.Candidate, Stability: candidate.Stability}
	baseAggregate := newEvidenceAccumulator()
	stressNetPnL := 0.0
	positiveFolds := 0

	byFold := map[protocolv2.FoldID]*DevelopmentFoldReview{}
	for _, checkpoint := range development.Units {
		unit := checkpoint.Unit
		if unit.Strategy.String() != candidate.Strategy.String() || unit.Control != "" || !strings.HasSuffix(string(unit.Fold), "-test") {
			continue
		}
		result, err := readCheckpointArtifact(root, checkpoint)
		if err != nil {
			return DevelopmentStrategyReview{}, err
		}
		summary := summarizeUnitEvidence(result, m.Risk.InitialEquity)
		fold := protocolv2.FoldID(strings.TrimSuffix(string(unit.Fold), "-test"))
		item := byFold[fold]
		if item == nil {
			item = &DevelopmentFoldReview{Fold: fold, Candidate: unit.Candidate}
			byFold[fold] = item
		}
		switch unit.Cost {
		case m.Execution.ID:
			item.Base = summary
			baseAggregate.add(result, m.Risk.InitialEquity)
			if summary.NetPnL > 0 {
				positiveFolds++
			}
		case "stress":
			item.Stress = summary
			stressNetPnL += summary.NetPnL
		}
		releaseResearchMemory()
	}
	for _, fold := range byFold {
		review.Folds = append(review.Folds, *fold)
	}
	sort.Slice(review.Folds, func(i, j int) bool { return review.Folds[i].Fold < review.Folds[j].Fold })

	aggregate := baseAggregate.summary()
	neighborsRobust, err := neighboringCandidatesRobust(m, development.Selections, candidate.Strategy)
	if err != nil {
		return DevelopmentStrategyReview{}, err
	}
	positiveFraction := 0.0
	if len(review.Folds) > 0 {
		positiveFraction = float64(positiveFolds) / float64(len(review.Folds))
	}
	review.Aggregate = DevelopmentAggregateReview{
		AggregateTrades: aggregate.ClosedTrades, EligibleSymbols: aggregate.EligibleSymbols,
		DevelopmentFolds: len(review.Folds), PositiveFoldFraction: positiveFraction,
		ProfitFactor: aggregate.ProfitFactor, Expectancy: aggregate.Expectancy,
		MedianDrawdownPercent: aggregate.MedianDrawdownPercent, MaxContributionPct: aggregate.MaxContributionPct,
		StressNetPnL: stressNetPnL, StressPositive: stressNetPnL > 0,
		ParameterStable: candidate.Stability.Stable, NeighborsRobust: neighborsRobust,
	}
	review.IrreversibleFailedGates = irreversibleDevelopmentGateFailures(m, review.Aggregate)
	review.PreHoldoutGateFlags = preHoldoutGateFlags(m, review.Aggregate)
	return review, nil
}

func irreversibleDevelopmentGateFailures(m manifest.Manifest, in DevelopmentAggregateReview) []string {
	var failed []string
	if in.DevelopmentFolds < m.Gates.MinDevelopmentFolds {
		failed = append(failed, "development_folds")
	}
	if in.PositiveFoldFraction < m.Gates.MinPositiveFoldFraction {
		failed = append(failed, "positive_fold_fraction")
	}
	if m.Gates.RequireParameterStability && !in.ParameterStable {
		failed = append(failed, "parameter_stability")
	}
	if !in.NeighborsRobust {
		failed = append(failed, "neighbor_sensitivity")
	}
	return failed
}

func preHoldoutGateFlags(m manifest.Manifest, in DevelopmentAggregateReview) []string {
	failed := irreversibleDevelopmentGateFailures(m, in)
	if in.AggregateTrades < m.Gates.MinAggregateTrades {
		failed = append(failed, "aggregate_trades")
	}
	if in.EligibleSymbols < m.Gates.MinEligibleSymbols {
		failed = append(failed, "eligible_symbols")
	}
	if in.ProfitFactor < m.Gates.MinProfitFactor {
		failed = append(failed, "profit_factor")
	}
	if m.Gates.RequirePositiveExpectancy && in.Expectancy <= 0 {
		failed = append(failed, "positive_expectancy")
	}
	if in.MedianDrawdownPercent > m.Gates.MaxMedianDrawdownPercent {
		failed = append(failed, "median_drawdown_percent")
	}
	if in.MaxContributionPct > m.Gates.MaxContributionPercent {
		failed = append(failed, "max_contribution_percent")
	}
	if m.Gates.RequireStressPositive && !in.StressPositive {
		failed = append(failed, "stress_positive")
	}
	return failed
}

type evidenceSummary struct {
	NetPnL                float64
	ClosedTrades          int
	EligibleSymbols       int
	PositiveSymbols       int
	ProfitFactor          float64
	Expectancy            float64
	MedianDrawdownPercent float64
	MaxContributionPct    float64
}

type evidenceAccumulator struct {
	symbols       map[protocolv2.Symbol]struct{}
	drawdowns     map[protocolv2.Symbol]float64
	contributions map[protocolv2.Symbol]float64
	netPnL        float64
	wins          float64
	losses        float64
	tradePnL      float64
	closedTrades  int
}

func newEvidenceAccumulator() *evidenceAccumulator {
	return &evidenceAccumulator{
		symbols: map[protocolv2.Symbol]struct{}{}, drawdowns: map[protocolv2.Symbol]float64{},
		contributions: map[protocolv2.Symbol]float64{},
	}
}

func (a *evidenceAccumulator) add(result UnitResult, initialEquity float64) {
	for _, artifact := range result.Symbols {
		a.symbols[artifact.Symbol] = struct{}{}
		pnl := artifact.FinalEquity - initialEquity
		a.netPnL += pnl
		a.contributions[artifact.Symbol] += pnl
		if drawdown := artifact.Metrics.MaxDrawdown * 100; drawdown > a.drawdowns[artifact.Symbol] {
			a.drawdowns[artifact.Symbol] = drawdown
		}
		for _, trade := range artifact.Trades {
			if trade.Status != execution.TradeClosed {
				continue
			}
			tradePnL := tradeNetPnL(trade)
			a.tradePnL += tradePnL
			a.closedTrades++
			if tradePnL > 0 {
				a.wins += tradePnL
			} else if tradePnL < 0 {
				a.losses -= tradePnL
			}
		}
	}
}

func (a *evidenceAccumulator) summary() evidenceSummary {
	drawdowns := make([]float64, 0, len(a.symbols))
	positiveSymbols := 0
	positiveContribution, largestContribution := 0.0, 0.0
	for symbol := range a.symbols {
		drawdowns = append(drawdowns, a.drawdowns[symbol])
		if contribution := a.contributions[symbol]; contribution > 0 {
			positiveSymbols++
			positiveContribution += contribution
			if contribution > largestContribution {
				largestContribution = contribution
			}
		}
	}
	profitFactor := 0.0
	if a.losses > 0 {
		profitFactor = a.wins / a.losses
	} else if a.wins > 0 {
		profitFactor = a.wins / 1e-8
	}
	expectancy := 0.0
	if a.closedTrades > 0 {
		expectancy = a.tradePnL / float64(a.closedTrades)
	}
	concentration := 0.0
	if positiveContribution > 0 {
		concentration = largestContribution / positiveContribution * 100
	}
	return evidenceSummary{
		NetPnL: a.netPnL, ClosedTrades: a.closedTrades, EligibleSymbols: len(a.symbols),
		PositiveSymbols: positiveSymbols, ProfitFactor: profitFactor, Expectancy: expectancy,
		MedianDrawdownPercent: medianFloat(drawdowns), MaxContributionPct: concentration,
	}
}

func summarizeUnitEvidence(result UnitResult, initialEquity float64) UnitEvidenceSummary {
	accumulator := newEvidenceAccumulator()
	accumulator.add(result, initialEquity)
	summary := accumulator.summary()
	return UnitEvidenceSummary{
		Unit: result.Unit, NetPnL: summary.NetPnL, ClosedTrades: summary.ClosedTrades,
		EligibleSymbols: summary.EligibleSymbols, PositiveSymbols: summary.PositiveSymbols,
		ProfitFactor: summary.ProfitFactor, Expectancy: summary.Expectancy,
		MedianDrawdownPercent: summary.MedianDrawdownPercent, MaxContributionPct: summary.MaxContributionPct,
	}
}

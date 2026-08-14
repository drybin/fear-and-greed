package orchestration

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
		summary, err := readCheckpointEvidence(root, checkpoint, m.Risk.InitialEquity, nil)
		if err != nil {
			return DevelopmentReview{}, err
		}
		review.Controls = append(review.Controls, summary)
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
		var aggregate *evidenceAccumulator
		if unit.Cost == m.Execution.ID {
			aggregate = baseAggregate
		}
		summary, err := readCheckpointEvidence(root, checkpoint, m.Risk.InitialEquity, aggregate)
		if err != nil {
			return DevelopmentStrategyReview{}, err
		}
		fold := protocolv2.FoldID(strings.TrimSuffix(string(unit.Fold), "-test"))
		item := byFold[fold]
		if item == nil {
			item = &DevelopmentFoldReview{Fold: fold, Candidate: unit.Candidate}
			byFold[fold] = item
		}
		switch unit.Cost {
		case m.Execution.ID:
			item.Base = summary
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
		a.addSymbolEvidence(artifact.Symbol, artifact.Metrics.MaxDrawdown, artifact.FinalEquity, artifact.Trades, initialEquity)
	}
}

func (a *evidenceAccumulator) addSymbolEvidence(symbol protocolv2.Symbol, maxDrawdown, finalEquity float64, trades []execution.TradeState, initialEquity float64) {
	a.symbols[symbol] = struct{}{}
	pnl := finalEquity - initialEquity
	a.netPnL += pnl
	a.contributions[symbol] += pnl
	if drawdown := maxDrawdown * 100; drawdown > a.drawdowns[symbol] {
		a.drawdowns[symbol] = drawdown
	}
	for _, trade := range trades {
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

// reviewSymbolArtifact contains precisely the fields required for a
// development decision. The decoder discards equity, audit, and rejection
// arrays instead of retaining an entire checkpoint in memory.
type reviewSymbolArtifact struct {
	Symbol      protocolv2.Symbol      `json:"symbol"`
	Metrics     reviewArtifactMetrics  `json:"metrics"`
	FinalEquity float64                `json:"final_equity"`
	Trades      []execution.TradeState `json:"trades"`
}

type reviewArtifactMetrics struct {
	MaxDrawdown float64 `json:"max_drawdown"`
}

// readCheckpointEvidence streams a checksum-protected checkpoint and retains
// only the compact data needed by review. Checkpoints can contain long equity
// curves and audit logs, so decoding UnitResult would otherwise retain all of
// them at once for every unit.
func readCheckpointEvidence(root string, checkpoint Checkpoint, initialEquity float64, aggregate *evidenceAccumulator) (UnitEvidenceSummary, error) {
	path := filepath.Join(protocolv2.CheckpointDir(root), checkpoint.Unit.Key()+".json.artifact")
	file, err := os.Open(path)
	if err != nil {
		return UnitEvidenceSummary{}, err
	}
	defer func() { _ = file.Close() }()

	hasher := sha256.New()
	buffered := bufio.NewReader(io.TeeReader(file, hasher))
	header, err := buffered.Peek(2)
	if err != nil && !errors.Is(err, io.EOF) {
		return UnitEvidenceSummary{}, err
	}

	var source io.Reader = buffered
	var compressed *gzip.Reader
	if len(header) == 2 && header[0] == 0x1f && header[1] == 0x8b {
		compressed, err = gzip.NewReader(buffered)
		if err != nil {
			return UnitEvidenceSummary{}, fmt.Errorf("orchestration: open compressed artifact: %w", err)
		}
		source = compressed
	}

	decoder := json.NewDecoder(source)
	start, err := decoder.Token()
	if err != nil || start != json.Delim('{') {
		return UnitEvidenceSummary{}, fmt.Errorf("orchestration: invalid checkpoint artifact JSON")
	}

	var unit Unit
	accumulator := newEvidenceAccumulator()
	for decoder.More() {
		field, err := decoder.Token()
		if err != nil {
			return UnitEvidenceSummary{}, fmt.Errorf("orchestration: read checkpoint artifact: %w", err)
		}
		name, ok := field.(string)
		if !ok {
			return UnitEvidenceSummary{}, fmt.Errorf("orchestration: invalid checkpoint artifact field")
		}
		switch name {
		case "unit":
			if err := decoder.Decode(&unit); err != nil {
				return UnitEvidenceSummary{}, fmt.Errorf("orchestration: decode checkpoint unit: %w", err)
			}
		case "symbols":
			if err := decodeReviewSymbols(decoder, accumulator, aggregate, initialEquity); err != nil {
				return UnitEvidenceSummary{}, err
			}
		default:
			if err := discardJSONValue(decoder); err != nil {
				return UnitEvidenceSummary{}, err
			}
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') {
		return UnitEvidenceSummary{}, fmt.Errorf("orchestration: invalid checkpoint artifact JSON")
	}
	if _, err := io.Copy(io.Discard, source); err != nil {
		return UnitEvidenceSummary{}, fmt.Errorf("orchestration: verify checkpoint payload: %w", err)
	}
	if compressed != nil {
		if err := compressed.Close(); err != nil {
			return UnitEvidenceSummary{}, fmt.Errorf("orchestration: close checkpoint decompressor: %w", err)
		}
	}
	if _, err := io.Copy(io.Discard, buffered); err != nil {
		return UnitEvidenceSummary{}, fmt.Errorf("orchestration: finish checkpoint checksum: %w", err)
	}
	actual := protocolv2.SHA256Hex(hex.EncodeToString(hasher.Sum(nil)))
	if actual != checkpoint.ArtifactSHA256 {
		return UnitEvidenceSummary{}, fmt.Errorf("artifact checksum mismatch")
	}
	if unit.Key() != checkpoint.Unit.Key() {
		return UnitEvidenceSummary{}, fmt.Errorf("orchestration: invalid checkpoint artifact for %s", checkpoint.Unit.Key())
	}

	summary := accumulator.summary()
	return UnitEvidenceSummary{
		Unit: unit, NetPnL: summary.NetPnL, ClosedTrades: summary.ClosedTrades,
		EligibleSymbols: summary.EligibleSymbols, PositiveSymbols: summary.PositiveSymbols,
		ProfitFactor: summary.ProfitFactor, Expectancy: summary.Expectancy,
		MedianDrawdownPercent: summary.MedianDrawdownPercent, MaxContributionPct: summary.MaxContributionPct,
	}, nil
}

func decodeReviewSymbols(decoder *json.Decoder, accumulator, aggregate *evidenceAccumulator, initialEquity float64) error {
	start, err := decoder.Token()
	if err != nil || start != json.Delim('[') {
		return fmt.Errorf("orchestration: checkpoint symbols must be an array")
	}
	for decoder.More() {
		var artifact reviewSymbolArtifact
		if err := decoder.Decode(&artifact); err != nil {
			return fmt.Errorf("orchestration: decode checkpoint symbol evidence: %w", err)
		}
		accumulator.addSymbolEvidence(artifact.Symbol, artifact.Metrics.MaxDrawdown, artifact.FinalEquity, artifact.Trades, initialEquity)
		if aggregate != nil {
			aggregate.addSymbolEvidence(artifact.Symbol, artifact.Metrics.MaxDrawdown, artifact.FinalEquity, artifact.Trades, initialEquity)
		}
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim(']') {
		return fmt.Errorf("orchestration: invalid checkpoint symbols")
	}
	return nil
}

func discardJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	for decoder.More() {
		if delim == '{' {
			if _, err := decoder.Token(); err != nil {
				return err
			}
		}
		if err := discardJSONValue(decoder); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

// Package orchestration owns the protocol-v2 phase boundary. It deliberately
// keeps execution behind Runner so tests and production adapters cannot bypass
// checkpoint, freeze, or holdout rules.
package orchestration

import (
	"context"
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

	"github.com/drybin/fear-and-greed/internal/research/candidates"
	"github.com/drybin/fear-and-greed/internal/research/controls"
	"github.com/drybin/fear-and-greed/internal/research/eligibility"
	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/drybin/fear-and-greed/internal/research/reporting"
	"github.com/drybin/fear-and-greed/internal/research/validation"
)

const (
	checkpointSchema = "protocol-v2.checkpoint.v1"
	freezeSchema     = "protocol-v2.freeze.v1"
)

var ErrHoldoutAccess = errors.New("orchestration: holdout access is forbidden outside final")

// Unit is the smallest independently resumable development work item.
type Unit struct {
	Strategy           protocolv2.StrategyRef          `json:"strategy"`
	Candidate          protocolv2.ParameterCandidateID `json:"candidate"`
	Fold               protocolv2.FoldID               `json:"fold"`
	Cost               protocolv2.CostProfileID        `json:"cost_profile"`
	Control            string                          `json:"control,omitempty"`
	ReferenceStrategy  protocolv2.StrategyRef          `json:"reference_strategy,omitempty"`
	ReferenceCandidate protocolv2.ParameterCandidateID `json:"reference_candidate,omitempty"`
	Symbols            []protocolv2.Symbol             `json:"symbols,omitempty"`
	Range              protocolv2.TimeRange            `json:"range"`
}

func (u Unit) Key() string {
	parts := []string{u.Strategy.String(), string(u.Candidate), string(u.Fold), string(u.Cost), u.Control, u.ReferenceStrategy.String(), string(u.ReferenceCandidate)}
	return strings.NewReplacer("/", "_", ":", "_", " ", "_").Replace(strings.Join(parts, "__"))
}

// Runner must only load observations within unit.Range. The orchestrator never
// creates a development unit over the locked holdout.
type Runner interface {
	Run(context.Context, Unit) ([]byte, error)
}

type Progress struct {
	Completed int
	Remaining int
	Reused    bool
	Unit      Unit
	Err       error
}

type DevelopmentOptions struct {
	Manifest             manifest.Manifest
	OutputDir            string
	SourceHash           protocolv2.SHA256Hex
	DataHash             protocolv2.SHA256Hex
	Runner               Runner
	Progress             func(Progress)
	AllowDirtyCode       bool
	CandleStore          CandleStore
	RequireDataHashMatch bool
	AllowUnverifiedData  bool // tests and synthetic runners only; CLI never enables it
}

type Checkpoint struct {
	SchemaVersion  string                  `json:"schema_version"`
	ExperimentID   protocolv2.ExperimentID `json:"experiment_id"`
	ManifestHash   protocolv2.SHA256Hex    `json:"manifest_hash"`
	SourceHash     protocolv2.SHA256Hex    `json:"source_hash"`
	DataHash       protocolv2.SHA256Hex    `json:"data_hash"`
	Unit           Unit                    `json:"unit"`
	ArtifactSHA256 protocolv2.SHA256Hex    `json:"artifact_sha256"`
	CompletedAt    time.Time               `json:"completed_at"`
}

type DevelopmentReport struct {
	ExperimentID protocolv2.ExperimentID `json:"experiment_id"`
	ManifestHash protocolv2.SHA256Hex    `json:"manifest_hash"`
	SourceHash   protocolv2.SHA256Hex    `json:"source_hash"`
	DataHash     protocolv2.SHA256Hex    `json:"data_hash"`
	Units        []Checkpoint            `json:"units"`
	Selections   []SelectionRecord       `json:"selections"`
}

type SelectionRecord struct {
	Selection validation.Selection        `json:"selection"`
	Scores    []validation.CandidateScore `json:"scores"`
}

// Development performs preflight, creates all candidate/fold/base+stress units,
// and atomically checkpoints each successful unit. Holdout is never handed to
// Runner during this phase.
func Development(ctx context.Context, options DevelopmentOptions) (DevelopmentReport, error) {
	if err := preflight(options); err != nil {
		return DevelopmentReport{}, err
	}
	folds, err := validation.GenerateFolds(options.Manifest.Schedule, 0)
	if err != nil {
		return DevelopmentReport{}, err
	}
	report := DevelopmentReport{ExperimentID: options.Manifest.ID, ManifestHash: options.Manifest.Hash, SourceHash: options.SourceHash, DataHash: options.DataHash}
	total := expectedDevelopmentUnits(options.Manifest, len(folds))
	completed := 0
	for _, strategy := range options.Manifest.Strategies {
		for _, fold := range folds {
			symbols, err := eligibleSymbols(options.Manifest, options.CandleStore, fold.Test, strategy.WarmupBars)
			if err != nil {
				return report, fmt.Errorf("orchestration: eligibility %s/%s: %w", strategy.Ref, fold.ID, err)
			}
			if len(symbols) == 0 {
				return report, fmt.Errorf("orchestration: no primary eligible symbols for %s/%s", strategy.Ref, fold.ID)
			}
			training := make([]validation.CandidateEvidence, 0, len(strategy.Grid))
			for _, candidate := range strategy.Grid {
				unit := Unit{Strategy: strategy.Ref, Candidate: candidate.ID, Fold: protocolv2.FoldID(string(fold.ID) + "-train"), Cost: options.Manifest.Execution.ID, Symbols: symbols, Range: fold.Train}
				checkpoint, artifact, reused, err := executeUnit(ctx, options, unit)
				if err != nil {
					return report, err
				}
				report.Units = append(report.Units, checkpoint)
				completed++
				notify(options.Progress, Progress{Completed: completed, Remaining: total - completed, Reused: reused, Unit: unit})
				evidence, err := candidateEvidence(candidate.ID, symbols, artifact, options.Manifest.Risk.InitialEquity)
				if err != nil {
					return report, err
				}
				training = append(training, evidence)
			}
			scores, err := validation.ScoreCandidates(symbols, training, validation.ScoreConfig{DrawdownPenalty: 0.1, LowSamplePenalty: 0.01, MinAggregateTrades: options.Manifest.Gates.MinAggregateTrades, ConcentrationPenalty: 0.01})
			if err != nil {
				return report, err
			}
			selection, err := validation.Select(strategy.Ref, fold.ID, scores)
			if err != nil {
				return report, err
			}
			report.Selections = append(report.Selections, SelectionRecord{Selection: selection, Scores: scores})

			for _, cost := range []protocolv2.CostProfileID{options.Manifest.Execution.ID, "stress"} {
				unit := Unit{Strategy: strategy.Ref, Candidate: selection.Candidate, Fold: protocolv2.FoldID(string(fold.ID) + "-test"), Cost: cost, Symbols: symbols, Range: fold.Test}
				checkpoint, _, reused, err := executeUnit(ctx, options, unit)
				if err != nil {
					return report, err
				}
				report.Units = append(report.Units, checkpoint)
				completed++
				notify(options.Progress, Progress{Completed: completed, Remaining: total - completed, Reused: reused, Unit: unit})
			}

			for _, control := range []string{string(controls.CashCode), string(controls.BuyAndHoldCode), string(controls.BTCBuyAndHoldCode), string(controls.EMA200Code), string(controls.RandomCode)} {
				unit := Unit{Strategy: protocolv2.StrategyRef{Code: protocolv2.StrategyCode(control), Version: controls.Version}, Candidate: "control", Fold: protocolv2.FoldID(string(fold.ID) + "-test"), Cost: options.Manifest.Execution.ID, Control: control, ReferenceStrategy: strategy.Ref, ReferenceCandidate: selection.Candidate, Symbols: symbols, Range: fold.Test}
				checkpoint, _, reused, err := executeUnit(ctx, options, unit)
				if err != nil {
					return report, err
				}
				report.Units = append(report.Units, checkpoint)
				completed++
				notify(options.Progress, Progress{Completed: completed, Remaining: total - completed, Reused: reused, Unit: unit})
			}
		}
	}
	sort.Slice(report.Units, func(i, j int) bool { return report.Units[i].Unit.Key() < report.Units[j].Unit.Key() })
	sort.Slice(report.Selections, func(i, j int) bool {
		left, right := report.Selections[i].Selection, report.Selections[j].Selection
		if left.Strategy.String() == right.Strategy.String() {
			return left.Fold < right.Fold
		}
		return left.Strategy.String() < right.Strategy.String()
	})
	if _, err := writeJSONAtomic(filepath.Join(protocolv2.ReportDir(experimentRoot(options.OutputDir, options.Manifest)), "development.json"), report); err != nil {
		return DevelopmentReport{}, err
	}
	return report, nil
}

func preflight(options DevelopmentOptions) error {
	if options.Runner == nil || strings.TrimSpace(options.OutputDir) == "" {
		return fmt.Errorf("orchestration: output directory and runner are required")
	}
	if err := options.Manifest.Validate(); err != nil {
		return err
	}
	if err := manifest.ValidateCoreStrategyCodes(options.Manifest.Strategies); err != nil {
		return err
	}
	if options.Manifest.ID == "" || options.Manifest.Hash == "" || options.SourceHash == "" || options.DataHash == "" {
		return fmt.Errorf("orchestration: frozen manifest, source hash, and data hash are required")
	}
	if err := os.MkdirAll(options.OutputDir, 0o755); err != nil {
		return fmt.Errorf("orchestration: output path: %w", err)
	}
	if options.CandleStore == nil && !options.AllowUnverifiedData {
		return fmt.Errorf("orchestration: candle store is required for eligibility and fingerprint verification")
	}
	if options.CandleStore != nil {
		dataHash, err := PreflightDevelopment(options.Manifest, options.OutputDir, options.CandleStore)
		if err != nil {
			return err
		}
		if options.DataHash != dataHash && options.RequireDataHashMatch {
			return fmt.Errorf("orchestration: data hash mismatch: got %s want %s", dataHash, options.DataHash)
		}
	}
	return nil
}

func expectedDevelopmentUnits(m manifest.Manifest, folds int) int {
	perFold := 0
	for _, strategy := range m.Strategies {
		perFold += len(strategy.Grid) + 2 + 5
	}
	return perFold * folds
}

func executeUnit(ctx context.Context, options DevelopmentOptions, unit Unit) (Checkpoint, []byte, bool, error) {
	if err := ctx.Err(); err != nil {
		return Checkpoint{}, nil, false, err
	}
	checkpoint, reused, err := reuseCheckpoint(options, unit)
	if err == nil && reused {
		artifact, readErr := os.ReadFile(checkpointPath(options, unit) + ".artifact")
		if readErr != nil {
			return Checkpoint{}, nil, false, readErr
		}
		if err := validateAndPersistUnitArtifact(options.Manifest.ID, protocolv2.ReportDir(experimentRoot(options.OutputDir, options.Manifest)), unit, artifact); err != nil {
			return Checkpoint{}, nil, false, err
		}
		return checkpoint, artifact, true, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Checkpoint{}, nil, false, fmt.Errorf("orchestration: checkpoint %s: %w", unit.Key(), err)
	}
	artifact, err := options.Runner.Run(ctx, unit)
	if err != nil {
		notify(options.Progress, Progress{Unit: unit, Err: err})
		return Checkpoint{}, nil, false, fmt.Errorf("orchestration: run %s: %w", unit.Key(), err)
	}
	if err := validateAndPersistUnitArtifact(options.Manifest.ID, protocolv2.ReportDir(experimentRoot(options.OutputDir, options.Manifest)), unit, artifact); err != nil {
		return Checkpoint{}, nil, false, err
	}
	checkpoint, err = writeCheckpoint(options, unit, artifact)
	return checkpoint, artifact, false, err
}

func candidateEvidence(id protocolv2.ParameterCandidateID, symbols []protocolv2.Symbol, raw []byte, initialEquity float64) (validation.CandidateEvidence, error) {
	var result UnitResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return validation.CandidateEvidence{}, fmt.Errorf("orchestration: decode candidate evidence: %w", err)
	}
	bySymbol := make(map[protocolv2.Symbol]UnitArtifact, len(result.Symbols))
	for _, artifact := range result.Symbols {
		bySymbol[artifact.Symbol] = artifact
	}
	evidence := validation.CandidateEvidence{Candidate: id, Symbols: make([]validation.SymbolEvidence, 0, len(symbols))}
	for _, symbol := range symbols {
		artifact, ok := bySymbol[symbol]
		if !ok {
			return validation.CandidateEvidence{}, fmt.Errorf("orchestration: candidate %s missing training evidence for %s", id, symbol)
		}
		evidence.Symbols = append(evidence.Symbols, validation.SymbolEvidence{Symbol: symbol, NetPnL: artifact.FinalEquity - initialEquity, MaxDrawdownPercent: artifact.Metrics.MaxDrawdown * 100, CompletedTrades: artifact.Metrics.ClosedTradeCount})
	}
	return evidence, nil
}

func eligibleSymbols(m manifest.Manifest, store CandleStore, test protocolv2.TimeRange, warmupBars int) ([]protocolv2.Symbol, error) {
	if store == nil {
		out := make([]protocolv2.Symbol, 0, len(m.Universe.Symbols))
		for _, symbol := range m.Universe.Symbols {
			out = append(out, symbol.Symbol)
		}
		return out, nil
	}
	snapshot := eligibility.FrozenSnapshot{Name: m.Universe.Name, AsOf: m.Cutoff, SourceFile: "manifest", Symbols: make([]protocolv2.Symbol, 0, len(m.Universe.Symbols))}
	inventories := make(map[protocolv2.Symbol]eligibility.FileInventory, len(m.Universe.Symbols))
	for _, symbol := range m.Universe.Symbols {
		snapshot.Symbols = append(snapshot.Symbols, symbol.Symbol)
		inventory, err := eligibility.InventoryFile(store.Path(symbol.Symbol))
		if err != nil {
			return nil, err
		}
		inventories[symbol.Symbol] = inventory
	}
	report, err := eligibility.EvaluateCohort(snapshot, inventories, test, warmupBars)
	if err != nil {
		return nil, err
	}
	out := make([]protocolv2.Symbol, 0, len(report.Primary))
	for _, item := range report.Primary {
		out = append(out, item.Symbol)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func experimentRoot(output string, m manifest.Manifest) string {
	return protocolv2.ExperimentRoot(output, m.ID)
}

func checkpointPath(options DevelopmentOptions, unit Unit) string {
	return filepath.Join(protocolv2.CheckpointDir(experimentRoot(options.OutputDir, options.Manifest)), unit.Key()+".json")
}

func reuseCheckpoint(options DevelopmentOptions, unit Unit) (Checkpoint, bool, error) {
	path := checkpointPath(options, unit)
	data, err := os.ReadFile(path)
	if err != nil {
		return Checkpoint{}, false, err
	}
	var checkpoint Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return Checkpoint{}, false, fmt.Errorf("invalid checkpoint: %w", err)
	}
	if checkpoint.SchemaVersion != checkpointSchema || checkpoint.ExperimentID != options.Manifest.ID || checkpoint.ManifestHash != options.Manifest.Hash || checkpoint.SourceHash != options.SourceHash || checkpoint.DataHash != options.DataHash || checkpoint.Unit.Key() != unit.Key() {
		return Checkpoint{}, false, fmt.Errorf("stale checkpoint")
	}
	artifact, err := os.ReadFile(path + ".artifact")
	if err != nil {
		return Checkpoint{}, false, err
	}
	if digest(artifact) != checkpoint.ArtifactSHA256 {
		return Checkpoint{}, false, fmt.Errorf("artifact checksum mismatch")
	}
	return checkpoint, true, nil
}

func writeCheckpoint(options DevelopmentOptions, unit Unit, artifact []byte) (Checkpoint, error) {
	path := checkpointPath(options, unit)
	checkpoint := Checkpoint{SchemaVersion: checkpointSchema, ExperimentID: options.Manifest.ID, ManifestHash: options.Manifest.Hash, SourceHash: options.SourceHash, DataHash: options.DataHash, Unit: unit, ArtifactSHA256: digest(artifact), CompletedAt: time.Now().UTC()}
	if err := writeAtomic(path+".artifact", artifact); err != nil {
		return Checkpoint{}, err
	}
	if _, err := writeJSONAtomic(path, checkpoint); err != nil {
		return Checkpoint{}, err
	}
	return checkpoint, nil
}

type FreezeBundle struct {
	SchemaVersion         string                  `json:"schema_version"`
	ExperimentID          protocolv2.ExperimentID `json:"experiment_id"`
	ManifestHash          protocolv2.SHA256Hex    `json:"manifest_hash"`
	SourceHash            protocolv2.SHA256Hex    `json:"source_hash"`
	DataHash              protocolv2.SHA256Hex    `json:"data_hash"`
	DevelopmentReportHash protocolv2.SHA256Hex    `json:"development_report_hash"`
	Manifest              manifest.Manifest       `json:"manifest"`
	FinalCandidates       []FinalCandidate        `json:"final_candidates"`
	CreatedAt             time.Time               `json:"created_at"`
}

type FinalCandidate struct {
	Strategy  protocolv2.StrategyRef          `json:"strategy"`
	Candidate protocolv2.ParameterCandidateID `json:"candidate"`
	Stability validation.ParameterStability   `json:"stability"`
}

func Freeze(outputDir string, m manifest.Manifest, sourceHash, dataHash protocolv2.SHA256Hex) (FreezeBundle, error) {
	root := experimentRoot(outputDir, m)
	reportPath := filepath.Join(protocolv2.ReportDir(root), "development.json")
	report, err := os.ReadFile(reportPath)
	if err != nil {
		return FreezeBundle{}, fmt.Errorf("orchestration: read development report: %w", err)
	}
	var development DevelopmentReport
	if err := json.Unmarshal(report, &development); err != nil || development.ExperimentID != m.ID || development.ManifestHash != m.Hash || development.SourceHash != sourceHash || development.DataHash != dataHash {
		return FreezeBundle{}, fmt.Errorf("orchestration: development report does not match frozen inputs")
	}
	if err := validateDevelopmentCompleteness(root, m, development); err != nil {
		return FreezeBundle{}, err
	}
	finalCandidates, err := deriveFinalCandidates(m, development.Selections)
	if err != nil {
		return FreezeBundle{}, err
	}
	bundle := FreezeBundle{SchemaVersion: freezeSchema, ExperimentID: m.ID, ManifestHash: m.Hash, SourceHash: sourceHash, DataHash: dataHash, DevelopmentReportHash: digest(report), Manifest: m, FinalCandidates: finalCandidates, CreatedAt: time.Now().UTC()}
	path := filepath.Join(protocolv2.FreezeDir(root), "bundle.json")
	if existing, err := os.ReadFile(path); err == nil {
		var prior FreezeBundle
		if json.Unmarshal(existing, &prior) == nil && prior.ManifestHash == bundle.ManifestHash && prior.DevelopmentReportHash == bundle.DevelopmentReportHash {
			return prior, nil
		}
		return FreezeBundle{}, fmt.Errorf("orchestration: refusing to overwrite immutable freeze bundle")
	}
	if _, err := writeJSONAtomic(path, bundle); err != nil {
		return FreezeBundle{}, err
	}
	return bundle, nil
}

type FinalUnitArtifact struct {
	Unit           Unit                 `json:"unit"`
	ArtifactSHA256 protocolv2.SHA256Hex `json:"artifact_sha256"`
	Path           string               `json:"path"`
}

type FinalStrategyDecision struct {
	Strategy  protocolv2.StrategyRef          `json:"strategy"`
	Candidate protocolv2.ParameterCandidateID `json:"candidate"`
	Input     validation.GateInput            `json:"gate_input"`
	Decision  validation.Decision             `json:"decision"`
}

type FinalReport struct {
	SchemaVersion string                  `json:"schema_version"`
	ExperimentID  protocolv2.ExperimentID `json:"experiment_id"`
	CompletedAt   time.Time               `json:"completed_at"`
	Units         []FinalUnitArtifact     `json:"units"`
	Decisions     []FinalStrategyDecision `json:"decisions"`
}

// Final opens a valid freeze bundle once, evaluates every frozen strategy under
// base and stress costs, retains artifacts, and applies the frozen gates.
func Final(ctx context.Context, outputDir string, m manifest.Manifest, sourceHash, dataHash protocolv2.SHA256Hex, runner Runner, candles CandleStore) (FinalReport, error) {
	if runner == nil {
		return FinalReport{}, fmt.Errorf("orchestration: final runner is required")
	}
	root := experimentRoot(outputDir, m)
	bundlePath := filepath.Join(protocolv2.FreezeDir(root), "bundle.json")
	raw, err := os.ReadFile(bundlePath)
	if err != nil {
		return FinalReport{}, fmt.Errorf("orchestration: read freeze bundle: %w", err)
	}
	var bundle FreezeBundle
	if err := json.Unmarshal(raw, &bundle); err != nil || bundle.SchemaVersion != freezeSchema || bundle.ExperimentID != m.ID || bundle.ManifestHash != m.Hash || bundle.SourceHash != sourceHash || bundle.DataHash != dataHash {
		return FinalReport{}, fmt.Errorf("orchestration: invalid or stale freeze bundle")
	}
	developmentRaw, err := os.ReadFile(filepath.Join(protocolv2.ReportDir(root), "development.json"))
	if err != nil || digest(developmentRaw) != bundle.DevelopmentReportHash {
		return FinalReport{}, fmt.Errorf("orchestration: frozen development report is missing or changed")
	}
	var development DevelopmentReport
	if err := json.Unmarshal(developmentRaw, &development); err != nil {
		return FinalReport{}, fmt.Errorf("orchestration: invalid development report: %w", err)
	}
	opening := filepath.Join(protocolv2.HoldoutDir(root), "opened.json")
	if err := writeJSONExclusive(opening, map[string]any{"experiment_id": m.ID, "opened_at": time.Now().UTC(), "bundle_sha256": digest(raw)}); err != nil {
		if errors.Is(err, os.ErrExist) {
			return FinalReport{}, fmt.Errorf("orchestration: holdout was already opened")
		}
		return FinalReport{}, err
	}

	final := FinalReport{SchemaVersion: "protocol-v2.final.v1", ExperimentID: m.ID}
	results := make(map[string]map[protocolv2.CostProfileID]UnitResult)
	for _, candidate := range bundle.FinalCandidates {
		strategyConfig, ok := manifestStrategy(m, candidate.Strategy)
		if !ok {
			return FinalReport{}, fmt.Errorf("orchestration: frozen strategy %s missing from manifest", candidate.Strategy)
		}
		symbols, err := eligibleSymbols(m, candles, m.Schedule.LockedHoldout, strategyConfig.WarmupBars)
		if err != nil {
			return FinalReport{}, err
		}
		if len(symbols) == 0 {
			return FinalReport{}, fmt.Errorf("orchestration: no primary eligible holdout symbols for %s", candidate.Strategy)
		}
		results[candidate.Strategy.String()] = make(map[protocolv2.CostProfileID]UnitResult)
		for _, cost := range []protocolv2.CostProfileID{m.Execution.ID, "stress"} {
			unit := Unit{Strategy: candidate.Strategy, Candidate: candidate.Candidate, Fold: "holdout", Cost: cost, Symbols: symbols, Range: m.Schedule.LockedHoldout}
			artifact, err := runner.Run(ctx, unit)
			if err != nil {
				return FinalReport{}, fmt.Errorf("orchestration: final %s: %w", unit.Key(), err)
			}
			var typed UnitResult
			if err := json.Unmarshal(artifact, &typed); err != nil || validateUnitResult(unit, typed) != nil {
				return FinalReport{}, fmt.Errorf("orchestration: invalid final artifact for %s", unit.Key())
			}
			if err := persistUnitReports(m.ID, filepath.Join(protocolv2.HoldoutDir(root), "reports"), typed); err != nil {
				return FinalReport{}, err
			}
			path := filepath.Join(protocolv2.HoldoutDir(root), unit.Key()+".json")
			hash, err := writeJSONAtomic(path, typed)
			if err != nil {
				return FinalReport{}, err
			}
			final.Units = append(final.Units, FinalUnitArtifact{Unit: unit, ArtifactSHA256: hash, Path: path})
			results[candidate.Strategy.String()][cost] = typed
		}
	}
	for _, candidate := range bundle.FinalCandidates {
		input, err := buildGateInput(root, m, development, candidate, results[candidate.Strategy.String()])
		if err != nil {
			return FinalReport{}, err
		}
		decision, err := validation.EvaluateGates(m.Gates, input)
		if err != nil {
			return FinalReport{}, err
		}
		final.Decisions = append(final.Decisions, FinalStrategyDecision{Strategy: candidate.Strategy, Candidate: candidate.Candidate, Input: input, Decision: decision})
	}
	final.CompletedAt = time.Now().UTC()
	if _, err := writeJSONAtomic(filepath.Join(protocolv2.HoldoutDir(root), "final.json"), final); err != nil {
		return FinalReport{}, err
	}
	return final, nil
}

func validateAndPersistUnitArtifact(experimentID protocolv2.ExperimentID, reportDir string, unit Unit, artifact []byte) error {
	var typed UnitResult
	if err := json.Unmarshal(artifact, &typed); err != nil || validateUnitResult(unit, typed) != nil {
		return fmt.Errorf("orchestration: runner returned an invalid unit artifact for %s", unit.Key())
	}
	return persistUnitReports(experimentID, reportDir, typed)
}

func validateUnitResult(unit Unit, result UnitResult) error {
	if result.Unit.Key() != unit.Key() || result.SymbolsEvaluated != len(result.Symbols) {
		return fmt.Errorf("unit identity or symbol count mismatch")
	}
	seen := make(map[protocolv2.Symbol]struct{}, len(result.Symbols))
	trades, rejections := 0, 0
	for _, artifact := range result.Symbols {
		if artifact.Unit.Key() != unit.Key() || artifact.Symbol.Validate() != nil {
			return fmt.Errorf("invalid symbol artifact identity")
		}
		if _, duplicate := seen[artifact.Symbol]; duplicate {
			return fmt.Errorf("duplicate symbol artifact")
		}
		seen[artifact.Symbol] = struct{}{}
		if artifact.TradeCount != len(artifact.Trades) || artifact.RejectionCount != len(artifact.Rejections) {
			return fmt.Errorf("symbol artifact evidence count mismatch")
		}
		trades += artifact.TradeCount
		rejections += artifact.RejectionCount
	}
	if trades != result.AggregateTradeCount || rejections != result.AggregateRejections {
		return fmt.Errorf("aggregate evidence count mismatch")
	}
	return nil
}

func persistUnitReports(experimentID protocolv2.ExperimentID, reportDir string, result UnitResult) error {
	for _, artifact := range result.Symbols {
		dir := filepath.Join(reportDir, "units", result.Unit.Key(), string(artifact.Symbol))
		fold := reporting.FoldReport{
			ArtifactHeader: reporting.Header(reporting.FoldSchemaVersion), ExperimentID: experimentID,
			FoldID: result.Unit.Fold, CandidateID: result.Unit.Candidate, Strategy: result.Unit.Strategy,
			Range: result.Unit.Range, Metrics: artifact.Metrics,
		}
		trades := reporting.TradeReport{ArtifactHeader: reporting.Header(reporting.TradeSchemaVersion), ExperimentID: experimentID, FoldID: result.Unit.Fold, Trades: artifact.Trades}
		fills := reporting.FillReport{ArtifactHeader: reporting.Header(reporting.FillSchemaVersion), ExperimentID: experimentID, FoldID: result.Unit.Fold, Fills: reportFills(artifact.Trades)}
		equity := reporting.EquityReport{ArtifactHeader: reporting.Header(reporting.EquitySchemaVersion), ExperimentID: experimentID, FoldID: result.Unit.Fold, Equity: artifact.Equity}
		rejections := reporting.RejectionReport{ArtifactHeader: reporting.Header(reporting.RejectionSchemaVersion), ExperimentID: experimentID, FoldID: result.Unit.Fold, Rejections: artifact.Rejections}
		for name, report := range map[string]any{"fold.json": fold, "trades.json": trades, "fills.json": fills, "equity.json": equity, "rejections.json": rejections} {
			if _, err := reporting.WriteJSON(filepath.Join(dir, name), report); err != nil {
				return fmt.Errorf("orchestration: write report %s/%s: %w", result.Unit.Key(), name, err)
			}
		}
	}
	return nil
}

func reportFills(trades []execution.TradeState) []reporting.FillRecord {
	var fills []reporting.FillRecord
	for i := range trades {
		trade := &trades[i]
		fills = append(fills, reporting.FillRecord{Kind: "entry", Entry: &trade.Entry})
		for j := range trade.PartialExits {
			fills = append(fills, reporting.FillRecord{Kind: "partial_exit", PartialExit: &trade.PartialExits[j]})
		}
		if trade.FinalExit != nil {
			fills = append(fills, reporting.FillRecord{Kind: "final_exit", FinalExit: trade.FinalExit})
		}
	}
	return fills
}

func validateDevelopmentCompleteness(root string, m manifest.Manifest, report DevelopmentReport) error {
	folds, err := validation.GenerateFolds(m.Schedule, 0)
	if err != nil {
		return err
	}
	expectedUnits := expectedDevelopmentUnits(m, len(folds))
	expectedSelections := len(m.Strategies) * len(folds)
	if len(report.Units) != expectedUnits || len(report.Selections) != expectedSelections {
		return fmt.Errorf("orchestration: development is incomplete: got %d/%d units and %d/%d selections", len(report.Units), expectedUnits, len(report.Selections), expectedSelections)
	}
	selections := make(map[string]validation.Selection, expectedSelections)
	for _, record := range report.Selections {
		selection := record.Selection
		key := selection.Strategy.String() + "__" + string(selection.Fold)
		if _, duplicate := selections[key]; duplicate || selection.Candidate != selection.Score.Candidate {
			return fmt.Errorf("orchestration: invalid or duplicate development selection %s", key)
		}
		selections[key] = selection
	}
	expected := make(map[string]protocolv2.TimeRange, expectedUnits)
	for _, strategy := range m.Strategies {
		candidateIDs := make(map[protocolv2.ParameterCandidateID]struct{}, len(strategy.Grid))
		for _, candidate := range strategy.Grid {
			candidateIDs[candidate.ID] = struct{}{}
		}
		for _, fold := range folds {
			selection, ok := selections[strategy.Ref.String()+"__"+string(fold.ID)]
			if !ok || selection.Strategy.String() != strategy.Ref.String() {
				return fmt.Errorf("orchestration: missing selection for %s/%s", strategy.Ref, fold.ID)
			}
			if _, ok := candidateIDs[selection.Candidate]; !ok {
				return fmt.Errorf("orchestration: selected candidate %s is outside grid for %s", selection.Candidate, strategy.Ref)
			}
			for _, candidate := range strategy.Grid {
				unit := Unit{Strategy: strategy.Ref, Candidate: candidate.ID, Fold: protocolv2.FoldID(string(fold.ID) + "-train"), Cost: m.Execution.ID, Range: fold.Train}
				expected[unit.Key()] = unit.Range
			}
			for _, cost := range []protocolv2.CostProfileID{m.Execution.ID, "stress"} {
				unit := Unit{Strategy: strategy.Ref, Candidate: selection.Candidate, Fold: protocolv2.FoldID(string(fold.ID) + "-test"), Cost: cost, Range: fold.Test}
				expected[unit.Key()] = unit.Range
			}
			for _, control := range []string{string(controls.CashCode), string(controls.BuyAndHoldCode), string(controls.BTCBuyAndHoldCode), string(controls.EMA200Code), string(controls.RandomCode)} {
				unit := Unit{Strategy: protocolv2.StrategyRef{Code: protocolv2.StrategyCode(control), Version: controls.Version}, Candidate: "control", Fold: protocolv2.FoldID(string(fold.ID) + "-test"), Cost: m.Execution.ID, Control: control, ReferenceStrategy: strategy.Ref, ReferenceCandidate: selection.Candidate, Range: fold.Test}
				expected[unit.Key()] = unit.Range
			}
		}
	}
	if len(expected) != expectedUnits {
		return fmt.Errorf("orchestration: expected development unit plan contains duplicate keys")
	}
	seen := make(map[string]struct{}, len(report.Units))
	for _, checkpoint := range report.Units {
		key := checkpoint.Unit.Key()
		if _, duplicate := seen[key]; duplicate {
			return fmt.Errorf("orchestration: duplicate development unit %s", key)
		}
		seen[key] = struct{}{}
		expectedRange, ok := expected[key]
		if !ok || checkpoint.Unit.Range != expectedRange {
			return fmt.Errorf("orchestration: unexpected development unit %s", key)
		}
		if checkpoint.SchemaVersion != checkpointSchema || checkpoint.ExperimentID != report.ExperimentID || checkpoint.ManifestHash != report.ManifestHash || checkpoint.SourceHash != report.SourceHash || checkpoint.DataHash != report.DataHash {
			return fmt.Errorf("orchestration: stale development checkpoint metadata for %s", key)
		}
		artifact, err := os.ReadFile(filepath.Join(protocolv2.CheckpointDir(root), key+".json.artifact"))
		if err != nil {
			return fmt.Errorf("orchestration: development artifact %s: %w", key, err)
		}
		if digest(artifact) != checkpoint.ArtifactSHA256 {
			return fmt.Errorf("orchestration: development artifact checksum mismatch for %s", key)
		}
		var typed UnitResult
		if err := json.Unmarshal(artifact, &typed); err != nil || typed.Unit.Key() != key {
			return fmt.Errorf("orchestration: invalid development artifact for %s", key)
		}
	}
	return nil
}

func deriveFinalCandidates(m manifest.Manifest, records []SelectionRecord) ([]FinalCandidate, error) {
	result := make([]FinalCandidate, 0, len(m.Strategies))
	for _, strategy := range m.Strategies {
		var selections []validation.Selection
		for _, record := range records {
			if record.Selection.Strategy.String() == strategy.Ref.String() {
				selections = append(selections, record.Selection)
			}
		}
		stability, err := validation.AssessParameterStability(selections, 0.5)
		if err != nil {
			return nil, fmt.Errorf("orchestration: parameter stability %s: %w", strategy.Ref, err)
		}
		result = append(result, FinalCandidate{Strategy: strategy.Ref, Candidate: stability.MostFrequent, Stability: stability})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Strategy.String() < result[j].Strategy.String() })
	return result, nil
}

func manifestStrategy(m manifest.Manifest, ref protocolv2.StrategyRef) (manifest.Strategy, bool) {
	for _, strategy := range m.Strategies {
		if strategy.Ref.String() == ref.String() {
			return strategy, true
		}
	}
	return manifest.Strategy{}, false
}

func buildGateInput(root string, m manifest.Manifest, development DevelopmentReport, candidate FinalCandidate, holdout map[protocolv2.CostProfileID]UnitResult) (validation.GateInput, error) {
	base := make([]UnitResult, 0)
	stress := make([]UnitResult, 0)
	for _, checkpoint := range development.Units {
		unit := checkpoint.Unit
		if unit.Strategy.String() != candidate.Strategy.String() || !strings.HasSuffix(string(unit.Fold), "-test") || unit.Control != "" {
			continue
		}
		artifact, err := readCheckpointArtifact(root, checkpoint)
		if err != nil {
			return validation.GateInput{}, err
		}
		switch unit.Cost {
		case m.Execution.ID:
			base = append(base, artifact)
		case "stress":
			stress = append(stress, artifact)
		}
	}
	baseHoldout, ok := holdout[m.Execution.ID]
	if !ok {
		return validation.GateInput{}, fmt.Errorf("orchestration: missing base holdout for %s", candidate.Strategy)
	}
	stressHoldout, ok := holdout["stress"]
	if !ok {
		return validation.GateInput{}, fmt.Errorf("orchestration: missing stress holdout for %s", candidate.Strategy)
	}
	positiveFolds := 0
	for _, result := range base {
		if unitNetPnL(result, m.Risk.InitialEquity) > 0 {
			positiveFolds++
		}
	}
	allBase := append(append([]UnitResult(nil), base...), baseHoldout)
	aggregate := aggregateGateEvidence(allBase, m.Risk.InitialEquity)
	stress = append(stress, stressHoldout)
	stressPnL := 0.0
	for _, result := range stress {
		stressPnL += unitNetPnL(result, m.Risk.InitialEquity)
	}
	neighborsRobust, err := neighboringCandidatesRobust(m, development.Selections, candidate.Strategy)
	if err != nil {
		return validation.GateInput{}, err
	}
	positiveFraction := 0.0
	if len(base) > 0 {
		positiveFraction = float64(positiveFolds) / float64(len(base))
	}
	return validation.GateInput{
		AggregateTrades:       aggregate.trades,
		EligibleSymbols:       aggregate.symbols,
		DevelopmentFolds:      len(base),
		PositiveFoldFraction:  positiveFraction,
		ProfitFactor:          aggregate.profitFactor,
		Expectancy:            aggregate.expectancy,
		MedianDrawdownPercent: aggregate.medianDrawdown,
		MaxContributionPct:    aggregate.maxContribution,
		StressPositive:        stressPnL > 0,
		ParameterStable:       candidate.Stability.Stable,
		NeighborsRobust:       neighborsRobust,
		HoldoutConsistent:     unitNetPnL(baseHoldout, m.Risk.InitialEquity) > 0,
	}, nil
}

type gateAggregate struct {
	trades, symbols                 int
	profitFactor, expectancy        float64
	medianDrawdown, maxContribution float64
}

func aggregateGateEvidence(results []UnitResult, initialEquity float64) gateAggregate {
	uniqueSymbols := map[protocolv2.Symbol]struct{}{}
	drawdownBySymbol := map[protocolv2.Symbol]float64{}
	contributionBySymbol := map[protocolv2.Symbol]float64{}
	wins, losses, totalPnL := 0.0, 0.0, 0.0
	closed := 0
	for _, result := range results {
		for _, artifact := range result.Symbols {
			uniqueSymbols[artifact.Symbol] = struct{}{}
			if drawdown := artifact.Metrics.MaxDrawdown * 100; drawdown > drawdownBySymbol[artifact.Symbol] {
				drawdownBySymbol[artifact.Symbol] = drawdown
			}
			contributionBySymbol[artifact.Symbol] += artifact.FinalEquity - initialEquity
			for _, trade := range artifact.Trades {
				if trade.Status != execution.TradeClosed {
					continue
				}
				pnl := tradeNetPnL(trade)
				totalPnL += pnl
				closed++
				if pnl > 0 {
					wins += pnl
				} else if pnl < 0 {
					losses -= pnl
				}
			}
		}
	}
	drawdowns := make([]float64, 0, len(uniqueSymbols))
	positive, largest := 0.0, 0.0
	for symbol := range uniqueSymbols {
		drawdowns = append(drawdowns, drawdownBySymbol[symbol])
		if contribution := contributionBySymbol[symbol]; contribution > 0 {
			positive += contribution
			if contribution > largest {
				largest = contribution
			}
		}
	}
	profitFactor := 0.0
	if losses > 0 {
		profitFactor = wins / losses
	} else if wins > 0 {
		profitFactor = wins / 1e-8
	}
	expectancy := 0.0
	if closed > 0 {
		expectancy = totalPnL / float64(closed)
	}
	concentration := 0.0
	if positive > 0 {
		concentration = largest / positive * 100
	}
	return gateAggregate{trades: closed, symbols: len(uniqueSymbols), profitFactor: profitFactor, expectancy: expectancy, medianDrawdown: medianFloat(drawdowns), maxContribution: concentration}
}

func tradeNetPnL(trade execution.TradeState) float64 {
	entry := trade.Entry.Price*trade.Entry.Quantity + trade.Entry.Commission
	proceeds, exitCommissions := 0.0, 0.0
	for _, fill := range trade.PartialExits {
		proceeds += fill.Price * fill.Quantity
		exitCommissions += fill.Commission
	}
	if trade.FinalExit != nil {
		proceeds += trade.FinalExit.Price * trade.FinalExit.Quantity
		exitCommissions += trade.FinalExit.Commission
	}
	return proceeds - entry - exitCommissions
}

func unitNetPnL(result UnitResult, initialEquity float64) float64 {
	total := 0.0
	for _, artifact := range result.Symbols {
		total += artifact.FinalEquity - initialEquity
	}
	return total
}

func neighboringCandidatesRobust(m manifest.Manifest, records []SelectionRecord, strategy protocolv2.StrategyRef) (bool, error) {
	config, ok := manifestStrategy(m, strategy)
	if !ok {
		return false, fmt.Errorf("orchestration: missing strategy grid for %s", strategy)
	}
	grid := make([]protocolv2.ParameterCandidateID, 0, len(config.Grid))
	for _, candidate := range config.Grid {
		grid = append(grid, candidate.ID)
	}
	for _, record := range records {
		if record.Selection.Strategy.String() != strategy.String() {
			continue
		}
		sensitivity, err := validation.NeighboringSensitivity(grid, record.Scores, record.Selection.Candidate, 50)
		if err != nil {
			return false, fmt.Errorf("orchestration: neighboring sensitivity %s/%s: %w", strategy, record.Selection.Fold, err)
		}
		for _, neighbor := range sensitivity {
			if neighbor.MaterialCollapse {
				return false, nil
			}
		}
	}
	return true, nil
}

func readCheckpointArtifact(root string, checkpoint Checkpoint) (UnitResult, error) {
	path := filepath.Join(protocolv2.CheckpointDir(root), checkpoint.Unit.Key()+".json.artifact")
	raw, err := os.ReadFile(path)
	if err != nil {
		return UnitResult{}, err
	}
	if digest(raw) != checkpoint.ArtifactSHA256 {
		return UnitResult{}, fmt.Errorf("orchestration: checkpoint artifact changed for %s", checkpoint.Unit.Key())
	}
	var result UnitResult
	if err := json.Unmarshal(raw, &result); err != nil || result.Unit.Key() != checkpoint.Unit.Key() {
		return UnitResult{}, fmt.Errorf("orchestration: invalid checkpoint artifact for %s", checkpoint.Unit.Key())
	}
	return result, nil
}

func medianFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 0 {
		return (values[middle-1] + values[middle]) / 2
	}
	return values[middle]
}

func writeJSONExclusive(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if _, err := file.Write(data); err != nil {
		cleanup()
		return err
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func digest(data []byte) protocolv2.SHA256Hex {
	sum := sha256.Sum256(data)
	return protocolv2.SHA256Hex(hex.EncodeToString(sum[:]))
}

func notify(callback func(Progress), progress Progress) {
	if callback != nil {
		callback(progress)
	}
}

func writeJSONAtomic(path string, value any) (protocolv2.SHA256Hex, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	return digest(data), writeAtomic(path, data)
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// Keep the import anchored: Core documents the intended four-adapter scope in
// this package and catches accidental expansion during compilation.
var _ = candidates.Core
var _ io.Reader

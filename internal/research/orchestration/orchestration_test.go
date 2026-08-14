package orchestration_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/orchestration"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

type runner struct {
	calls int
	fail  int
	seen  []orchestration.Unit
}

func (r *runner) Run(_ context.Context, unit orchestration.Unit) ([]byte, error) {
	r.calls++
	r.seen = append(r.seen, unit)
	if r.fail == r.calls {
		return nil, errors.New("interrupted")
	}
	symbols := unit.Symbols
	if len(symbols) == 0 {
		symbols = []protocolv2.Symbol{"BTCUSDT"}
	}
	artifacts := make([]orchestration.UnitArtifact, 0, len(symbols))
	for _, symbol := range symbols {
		artifacts = append(artifacts, orchestration.UnitArtifact{Unit: unit, Symbol: symbol, FinalEquity: 1010})
	}
	return json.Marshal(orchestration.UnitResult{Unit: unit, Symbols: artifacts, SymbolsEvaluated: len(artifacts)})
}

func TestDevelopmentResumesAndRejectsStaleCheckpoint(t *testing.T) {
	m := fixtureManifest(t)
	dir := t.TempDir()
	first := &runner{fail: 2}
	_, err := orchestration.Development(context.Background(), options(m, dir, first, "source-a"))
	if err == nil {
		t.Fatal("expected interruption")
	}
	second := &runner{}
	report, err := orchestration.Development(context.Background(), options(m, dir, second, "source-a"))
	if err != nil {
		t.Fatal(err)
	}
	if second.calls != len(report.Units)-1 {
		t.Fatalf("resume ran %d units, want %d", second.calls, len(report.Units)-1)
	}
	artifactPath := filepath.Join(protocolv2.CheckpointDir(protocolv2.ExperimentRoot(dir, m.ID)), report.Units[0].Unit.Key()+".json.artifact")
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifact) < 2 || artifact[0] != 0x1f || artifact[1] != 0x8b {
		t.Fatal("checkpoint artifact is not gzip-compressed")
	}
	if _, err := orchestration.Development(context.Background(), options(m, dir, &runner{}, "source-b")); err == nil {
		t.Fatal("expected stale checkpoint rejection")
	}
}

func TestDevelopmentNeverPassesHoldoutAndFinalResumesSameOpening(t *testing.T) {
	m := fixtureManifest(t)
	dir := t.TempDir()
	development := &runner{}
	report, err := orchestration.Development(context.Background(), options(m, dir, development, "source-a"))
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range development.seen {
		if !unit.Range.End.Before(m.Schedule.LockedHoldout.Start) && !unit.Range.End.Equal(m.Schedule.LockedHoldout.Start) {
			t.Fatalf("development received holdout range: %+v", unit.Range)
		}
	}
	if len(report.Units) == 0 {
		t.Fatal("expected checkpoints")
	}
	firstReport := filepath.Join(protocolv2.ReportDir(protocolv2.ExperimentRoot(dir, m.ID)), "units", report.Units[0].Unit.Key(), "BTCUSDT", "fold.json.sha256")
	if _, err := os.Stat(firstReport); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("development unexpectedly duplicated checkpoint reports: %v", err)
	}
	for _, unit := range development.seen {
		if strings.HasSuffix(string(unit.Fold), "-test") && unit.Control == "" && unit.Strategy.Code == m.Strategies[0].Ref.Code && unit.Candidate != "alternate" {
			t.Fatalf("test fold received unselected candidate %s", unit.Candidate)
		}
	}
	if _, err := orchestration.Freeze(dir, m, "source-a", "data-a"); err != nil {
		t.Fatal(err)
	}
	final := &runner{}
	finalReport, err := orchestration.Final(context.Background(), dir, m, "source-a", "data-a", final, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(final.seen) != len(m.Strategies)*2 || len(finalReport.Decisions) != len(m.Strategies) {
		t.Fatalf("final ran %d units and produced %d decisions", len(final.seen), len(finalReport.Decisions))
	}
	for _, unit := range final.seen {
		if unit.Range != m.Schedule.LockedHoldout {
			t.Fatal("final received a range outside the locked holdout")
		}
	}
	openedPath := filepath.Join(protocolv2.HoldoutDir(protocolv2.ExperimentRoot(dir, m.ID)), "opened.json")
	openedBefore, err := os.ReadFile(openedPath)
	if err != nil {
		t.Fatal(err)
	}
	secondReport, err := orchestration.Final(context.Background(), dir, m, "source-a", "data-a", final, nil)
	if err != nil {
		t.Fatal(err)
	}
	openedAfter, err := os.ReadFile(openedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(openedBefore) != string(openedAfter) || len(secondReport.Units) != len(finalReport.Units) {
		t.Fatal("resume changed the immutable opening or completed report")
	}
	if final.calls != len(m.Strategies)*2 {
		t.Fatalf("completed final reran evaluator: %d calls", final.calls)
	}
	finalPath := filepath.Join(protocolv2.HoldoutDir(protocolv2.ExperimentRoot(dir, m.ID)), "final.json")
	finalBytes, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(finalPath, append(finalBytes, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := orchestration.Final(context.Background(), dir, m, "source-a", "data-a", final, nil); err == nil {
		t.Fatal("expected changed final report checksum rejection")
	}
}

func TestFinalResumesCheckpointedUnitsAfterInterruption(t *testing.T) {
	m := fixtureManifest(t)
	dir := t.TempDir()
	if _, err := orchestration.Development(context.Background(), options(m, dir, &runner{}, "source-a")); err != nil {
		t.Fatal(err)
	}
	if _, err := orchestration.Freeze(dir, m, "source-a", "data-a"); err != nil {
		t.Fatal(err)
	}

	interrupted := &runner{fail: 2}
	if _, err := orchestration.Final(context.Background(), dir, m, "source-a", "data-a", interrupted, nil); err == nil {
		t.Fatal("expected final interruption")
	}
	root := protocolv2.ExperimentRoot(dir, m.ID)
	artifacts, err := filepath.Glob(filepath.Join(protocolv2.HoldoutDir(root), "checkpoints", "*.artifact"))
	if err != nil || len(artifacts) != 1 {
		t.Fatalf("expected one final checkpoint artifact: %v, %v", artifacts, err)
	}
	compressed, err := os.ReadFile(artifacts[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) < 2 || compressed[0] != 0x1f || compressed[1] != 0x8b {
		t.Fatal("final checkpoint artifact is not gzip-compressed")
	}
	openedBefore, err := os.ReadFile(filepath.Join(protocolv2.HoldoutDir(root), "opened.json"))
	if err != nil {
		t.Fatal(err)
	}

	resumed := &runner{}
	report, err := orchestration.Final(context.Background(), dir, m, "source-a", "data-a", resumed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.calls != len(m.Strategies)*2-1 {
		t.Fatalf("resume ran %d units, want %d", resumed.calls, len(m.Strategies)*2-1)
	}
	if len(report.Decisions) != len(m.Strategies) {
		t.Fatalf("resume produced %d decisions", len(report.Decisions))
	}
	openedAfter, err := os.ReadFile(filepath.Join(protocolv2.HoldoutDir(root), "opened.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(openedBefore) != string(openedAfter) {
		t.Fatal("resume rewrote holdout opening")
	}
}

func TestDevelopmentReviewDoesNotOpenHoldout(t *testing.T) {
	m := fixtureManifest(t)
	dir := t.TempDir()
	if _, err := orchestration.Development(context.Background(), options(m, dir, &runner{}, "source-a")); err != nil {
		t.Fatal(err)
	}
	review, err := orchestration.ReviewDevelopment(dir, m, "source-a", "data-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(review.Strategies) != len(m.Strategies) || len(review.Controls) == 0 {
		t.Fatalf("incomplete review: %d strategies, %d controls", len(review.Strategies), len(review.Controls))
	}
	for _, strategy := range review.Strategies {
		baseTrades := 0
		for _, fold := range strategy.Folds {
			baseTrades += fold.Base.ClosedTrades
		}
		if strategy.Aggregate.AggregateTrades != baseTrades {
			t.Fatalf("review aggregate includes non-base trades for %s: got %d, want %d", strategy.Strategy, strategy.Aggregate.AggregateTrades, baseTrades)
		}
	}
	opened := filepath.Join(protocolv2.HoldoutDir(protocolv2.ExperimentRoot(dir, m.ID)), "opened.json")
	if _, err := os.Stat(opened); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("review accessed holdout: %v", err)
	}
}

func TestFreezeRejectsChangedDevelopmentArtifact(t *testing.T) {
	m := fixtureManifest(t)
	dir := t.TempDir()
	report, err := orchestration.Development(context.Background(), options(m, dir, &runner{}, "source-a"))
	if err != nil {
		t.Fatal(err)
	}
	artifactPath := filepath.Join(protocolv2.CheckpointDir(protocolv2.ExperimentRoot(dir, m.ID)), report.Units[0].Unit.Key()+".json.artifact")
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, append(artifact, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := orchestration.Freeze(dir, m, "source-a", "data-a"); err == nil {
		t.Fatal("expected freeze to reject changed development evidence")
	}
}

func options(m manifest.Manifest, dir string, r *runner, source string) orchestration.DevelopmentOptions {
	return orchestration.DevelopmentOptions{Manifest: m, OutputDir: dir, SourceHash: protocolv2.SHA256Hex(source), DataHash: "data-a", Runner: r, AllowUnverifiedData: true}
}

func fixtureManifest(t *testing.T) manifest.Manifest {
	t.Helper()
	utc := time.UTC
	r := func(start, end time.Time) protocolv2.TimeRange {
		return protocolv2.TimeRange{Start: start.In(utc), End: end.In(utc)}
	}
	strategies := []manifest.Strategy{}
	for _, code := range []protocolv2.StrategyCode{"fib-pullback-trend-v1", "nr7-trend-breakout-v1", "volatility-compression-breakout-v1", "breakout-retest-long-v2"} {
		strategies = append(strategies, manifest.Strategy{Ref: protocolv2.StrategyRef{Code: code, Version: "v1"}, Timeframe: "1h", DefaultParams: map[string]any{}, Grid: []manifest.ParameterCandidate{{ID: "default", Values: map[string]any{}}}})
	}
	strategies[0].Grid = append(strategies[0].Grid, manifest.ParameterCandidate{ID: "alternate", Values: map[string]any{}})
	m := manifest.Manifest{
		SchemaVersion: protocolv2.ManifestSchemaVersion, ProtocolVersion: manifest.ProtocolVersion,
		Cutoff: time.Date(2023, 10, 1, 0, 0, 0, 0, utc), Source: manifest.SourceRevision{GitRevision: "test"}, Seed: 1,
		Universe:   manifest.UniverseSnapshot{Name: "fixture", Provenance: protocolv2.UniverseFrozenCurrentCohort, Exchange: "binance", Spot: true, QuoteAsset: "USDT", Symbols: []manifest.SymbolSnapshot{{Symbol: "BTCUSDT", CandleSHA256: protocolv2.SHA256Hex(strings.Repeat("a", 64))}}},
		Strategies: strategies,
		Schedule:   manifest.Schedule{Train: r(time.Date(2022, 1, 1, 0, 0, 0, 0, utc), time.Date(2022, 10, 1, 0, 0, 0, 0, utc)), Test: r(time.Date(2022, 10, 1, 0, 0, 0, 0, utc), time.Date(2023, 1, 1, 0, 0, 0, 0, utc)), FoldStep: 90 * 24 * time.Hour, LockedHoldout: r(time.Date(2023, 7, 1, 0, 0, 0, 0, utc), time.Date(2023, 10, 1, 0, 0, 0, 0, utc))},
		Execution:  manifest.ExecutionProfile{ID: "base", CommissionBPS: 10, SlippageBPS: 5, GapPolicy: manifest.GapReject, IntrabarPolicy: manifest.IntrabarStopFirst},
		Risk:       manifest.StandaloneRisk{SizingProfile: "standalone", InitialEquity: 1000, RiskPerTradePercent: 1, MaxNotionalPercent: 20},
		Gates:      manifest.Gates{MinAggregateTrades: 1, MinEligibleSymbols: 1, MinDevelopmentFolds: 3, MinPositiveFoldFraction: .6, MinProfitFactor: 1.15, RequirePositiveExpectancy: true, MaxMedianDrawdownPercent: 20, MaxContributionPercent: 25, RequireStressPositive: true, RequireParameterStability: true, RequireHoldoutConsistency: true},
	}
	if err := m.Freeze(); err != nil {
		t.Fatal(err)
	}
	return m
}

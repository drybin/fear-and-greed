package orchestration

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/candidates"
	"github.com/drybin/fear-and-greed/internal/research/eligibility"
	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

const researchFold = 90 * 24 * time.Hour

type PrepareManifestOptions struct {
	SymbolsFile string
	CandleDir   string
	OutputPath  string
	Cutoff      time.Time
	Source      manifest.SourceRevision
	Seed        uint64
	Suite       string
}

// PrepareManifest fingerprints the frozen cohort and creates the immutable
// core protocol-v2 manifest. Candle contents are never rewritten.
func PrepareManifest(options PrepareManifestOptions) (manifest.Manifest, error) {
	if options.SymbolsFile == "" || options.CandleDir == "" || options.OutputPath == "" {
		return manifest.Manifest{}, fmt.Errorf("orchestration: symbols, candle directory, and manifest output are required")
	}
	cutoff := options.Cutoff.UTC()
	if cutoff.IsZero() || options.Cutoff.Location() != time.UTC || cutoff.Hour() != 0 || cutoff.Minute() != 0 || cutoff.Second() != 0 || cutoff.Nanosecond() != 0 {
		return manifest.Manifest{}, fmt.Errorf("orchestration: cutoff must be UTC midnight")
	}
	if options.Source.GitRevision == "" || options.Source.Dirty {
		return manifest.Manifest{}, fmt.Errorf("orchestration: a clean Git source revision is required")
	}
	snapshot, err := eligibility.LoadFrozenSnapshot(options.SymbolsFile, "top-50-"+cutoff.Format("2006-01-02"), cutoff)
	if err != nil {
		return manifest.Manifest{}, err
	}
	if len(snapshot.Symbols) != 50 {
		return manifest.Manifest{}, fmt.Errorf("orchestration: core manifest requires exactly 50 frozen symbols, got %d", len(snapshot.Symbols))
	}

	symbols := make([]manifest.SymbolSnapshot, 0, len(snapshot.Symbols))
	for _, symbol := range snapshot.Symbols {
		inventory, err := eligibility.InventoryFile(filepath.Join(options.CandleDir, string(symbol)+".csv"))
		if err != nil {
			return manifest.Manifest{}, fmt.Errorf("orchestration: inventory %s: %w", symbol, err)
		}
		if !inventory.CoreUsable() {
			return manifest.Manifest{}, fmt.Errorf("orchestration: candle file for %s failed core quality checks", symbol)
		}
		if inventory.Interval != time.Minute {
			return manifest.Manifest{}, fmt.Errorf("orchestration: %s interval is %s, want 1m", symbol, inventory.Interval)
		}
		if inventory.Range.End.Before(cutoff) {
			return manifest.Manifest{}, fmt.Errorf("orchestration: %s ends at %s before cutoff %s", symbol, inventory.Range.End, cutoff)
		}
		symbols = append(symbols, manifest.SymbolSnapshot{Symbol: symbol, CandleSHA256: inventory.SHA256})
	}

	adapters, err := researchSuite(options.Suite)
	if err != nil {
		return manifest.Manifest{}, err
	}
	strategies := make([]manifest.Strategy, 0, len(adapters))
	for _, adapter := range adapters {
		metadata := adapter.Metadata()
		grid := make([]manifest.ParameterCandidate, 0, len(adapter.Grid()))
		for _, candidate := range adapter.Grid() {
			grid = append(grid, manifest.ParameterCandidate{ID: candidate.ID, Values: candidate.Values})
		}
		strategies = append(strategies, manifest.Strategy{Ref: metadata.Ref, Timeframe: metadata.Timeframe, WarmupBars: metadata.WarmupBars, DefaultParams: map[string]any{}, Grid: grid})
	}

	firstTestStart := cutoff.Add(-4 * researchFold)
	m := manifest.Manifest{
		SchemaVersion:   protocolv2.ManifestSchemaVersion,
		ProtocolVersion: manifest.ProtocolVersion,
		Cutoff:          cutoff,
		Source:          options.Source,
		Seed:            options.Seed,
		Universe: manifest.UniverseSnapshot{
			Name: snapshot.Name, Provenance: protocolv2.UniverseFrozenCurrentCohort,
			Exchange: "binance", Spot: true, QuoteAsset: "USDT", Symbols: symbols,
		},
		Strategies: strategies,
		Schedule: manifest.Schedule{
			Train:         protocolv2.TimeRange{Start: firstTestStart.Add(-3 * researchFold), End: firstTestStart},
			Test:          protocolv2.TimeRange{Start: firstTestStart, End: firstTestStart.Add(researchFold)},
			FoldStep:      researchFold,
			LockedHoldout: protocolv2.TimeRange{Start: cutoff.Add(-researchFold), End: cutoff},
		},
		Execution: manifest.ExecutionProfile{ID: "base", CommissionBPS: 10, SlippageBPS: 5, GapPolicy: manifest.GapReject, IntrabarPolicy: manifest.IntrabarStopFirst},
		Risk:      manifest.StandaloneRisk{SizingProfile: "one-percent", InitialEquity: 10000, RiskPerTradePercent: 1, MaxNotionalPercent: 20},
		Gates: manifest.Gates{
			MinAggregateTrades: 100, MinEligibleSymbols: 20, MinDevelopmentFolds: 3,
			MinPositiveFoldFraction: 0.6, MinProfitFactor: 1.15, RequirePositiveExpectancy: true,
			MaxMedianDrawdownPercent: 20, MaxContributionPercent: 25, RequireStressPositive: true,
			RequireParameterStability: true, RequireHoldoutConsistency: true,
		},
	}
	if err := m.Freeze(); err != nil {
		return manifest.Manifest{}, err
	}
	if err := manifest.ValidateSupportedStrategyCodes(m.Strategies); err != nil {
		return manifest.Manifest{}, err
	}
	if err := manifest.WriteFile(options.OutputPath, m); err != nil {
		return manifest.Manifest{}, err
	}
	return m, nil
}

func researchSuite(name string) ([]candidates.Adapter, error) {
	switch name {
	case "", "core-v2":
		return candidates.Core(), nil
	case "research-v3":
		return candidates.ResearchV3(), nil
	case "daily-low-zone-v1_1":
		return candidates.DailyLowZoneV11(), nil
	case "daily-low-zone-v1_2":
		return candidates.DailyLowZoneV12(), nil
	case "daily-low-zone-v1_3":
		return candidates.DailyLowZoneV13(), nil
	default:
		return nil, fmt.Errorf("orchestration: unknown research suite %q", name)
	}
}

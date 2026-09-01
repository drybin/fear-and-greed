package portfolio

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/drybin/fear-and-greed/internal/infrastructure/csvdata"
	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

func Prepare(sourcePath, outputPath, revision string, diagnostic bool, regimeMode RegimeMode, entryMode EntryMode) (Manifest, error) {
	raw, err := os.ReadFile(sourcePath)
	if err != nil {
		return Manifest{}, fmt.Errorf("portfolio: read source manifest: %w", err)
	}
	source, err := manifest.Decode(raw)
	if err != nil {
		return Manifest{}, err
	}
	m, err := DefaultManifest(source, revision, diagnostic, regimeMode, entryMode)
	if err != nil {
		return Manifest{}, err
	}
	if err := writeImmutableJSON(outputPath, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func Run(ctx context.Context, m Manifest, candleDir, outputPath string) (Report, error) {
	if err := m.Validate(); err != nil {
		return Report{}, err
	}
	if err := m.VerifyIdentity(); err != nil {
		return Report{}, err
	}
	if err := VerifySignalArtifacts(m.SignalArtifacts); err != nil {
		return Report{}, err
	}
	warmupDays := maxInt(m.RelativeStrength.ReturnLookbackDays+1, m.RelativeStrength.VolatilityDays+1, m.RelativeStrength.ATRDays+1, m.RelativeStrength.BTCEMADays+1)
	warmupStart := m.Range.Start.Add(-time.Duration(warmupDays+7) * 24 * time.Hour)
	bars := make(map[protocolv2.Symbol][]DailyBar, len(m.Universe.Symbols))
	for _, snapshot := range m.Universe.Symbols {
		if err := ctx.Err(); err != nil {
			return Report{}, err
		}
		path := filepath.Join(candleDir, string(snapshot.Symbol)+".csv")
		if err := verifyFileSHA256(ctx, path, snapshot.CandleSHA256); err != nil {
			return Report{}, err
		}
		minutes, err := csvdata.LoadKlinesRange(path, warmupStart, m.Range.End)
		if err != nil {
			return Report{}, fmt.Errorf("portfolio: load %s: %w", snapshot.Symbol, err)
		}
		daily, err := AggregateDaily(minutes)
		if err != nil {
			return Report{}, fmt.Errorf("portfolio: aggregate %s: %w", snapshot.Symbol, err)
		}
		bars[snapshot.Symbol] = daily
	}
	rebalances, err := RelativeStrengthRebalances(bars, m.RelativeStrength, m.Range.Start, m.Range.End)
	if err != nil {
		return Report{}, err
	}
	baseResult, err := (Engine{Limits: m.Limits, Costs: m.BaseCosts}).Run(bars, rebalances, m.Range.Start, m.Range.End)
	if err != nil {
		return Report{}, err
	}
	stressResult, err := (Engine{Limits: m.Limits, Costs: m.StressCosts}).Run(bars, rebalances, m.Range.Start, m.Range.End)
	if err != nil {
		return Report{}, err
	}
	baseMetrics, stressMetrics := CalculateMetrics(baseResult), CalculateMetrics(stressResult)
	benchmarks := BuildBenchmarks(bars, m.Range.Start, m.Range.End, m.Limits.InitialCapital, m.BaseCosts)
	report := Report{
		SchemaVersion: ReportSchemaVersion,
		ExperimentID:  m.ID,
		ManifestHash:  m.Hash,
		GeneratedAt:   m.Range.End,
		Strategy:      relativeStrengthStrategy(m.RelativeStrength.EntryMode),
		Candidate:     relativeStrengthCandidate(m.RelativeStrength.RegimeMode, m.RelativeStrength.EntryMode),
		Diagnostic:    m.Diagnostic,
		Base:          baseMetrics,
		Stress:        stressMetrics,
		Benchmarks:    benchmarks,
		Decision:      EvaluateDecision(baseMetrics, stressMetrics, benchmarks, m.Gates, m.Diagnostic),
		Rebalances:    rebalances,
		BaseResult:    baseResult,
		StressResult:  stressResult,
	}
	if err := report.Validate(); err != nil {
		return Report{}, err
	}
	if err := writeImmutableJSON(outputPath, report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func relativeStrengthCandidate(regimeMode RegimeMode, entryMode EntryMode) string {
	return "rs-90d-vol30-top5-" + string(regimeMode.normalized()) + "-" + string(entryMode.normalized())
}

func relativeStrengthStrategy(entryMode EntryMode) protocolv2.StrategyRef {
	code := StrategyCode
	if entryMode.normalized() == EntryModeTrendPullback {
		code = "relative-strength-pullback-v1"
	}
	return protocolv2.StrategyRef{Code: protocolv2.StrategyCode(code), Version: protocolv2.StrategyVersion(StrategyVersion)}
}

func (r Report) Validate() error {
	if r.SchemaVersion != ReportSchemaVersion {
		return fmt.Errorf("portfolio: invalid report schema")
	}
	if err := r.ExperimentID.Validate(); err != nil {
		return err
	}
	if err := r.ManifestHash.Validate(); err != nil {
		return err
	}
	if err := r.Strategy.Validate(); err != nil {
		return err
	}
	if r.GeneratedAt.IsZero() || r.Candidate == "" {
		return fmt.Errorf("portfolio: incomplete report identity")
	}
	if r.Decision.Status != "portfolio-pass" && r.Decision.Status != "observe" && r.Decision.Status != "reject" {
		return fmt.Errorf("portfolio: invalid decision %q", r.Decision.Status)
	}
	return nil
}

func verifyFileSHA256(ctx context.Context, path string, expected protocolv2.SHA256Hex) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("portfolio: open candle input: %w", err)
	}
	defer func() { _ = file.Close() }()
	hash := sha256.New()
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := file.Read(buffer)
		if n > 0 {
			if _, err := hash.Write(buffer[:n]); err != nil {
				return fmt.Errorf("portfolio: hash candle input: %w", err)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("portfolio: read candle input: %w", readErr)
		}
	}
	actual := protocolv2.SHA256Hex(hex.EncodeToString(hash.Sum(nil)))
	if actual != expected {
		return fmt.Errorf("portfolio: candle checksum mismatch: %s", path)
	}
	return nil
}

func writeImmutableJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("portfolio: marshal artifact: %w", err)
	}
	data = append(data, '\n')
	if existing, err := os.ReadFile(path); err == nil {
		if string(existing) != string(data) {
			return fmt.Errorf("portfolio: refusing to replace different artifact %s", path)
		}
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("portfolio: read existing artifact: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("portfolio: create artifact directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".portfolio-*.tmp")
	if err != nil {
		return fmt.Errorf("portfolio: create temporary artifact: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("portfolio: write temporary artifact: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("portfolio: sync temporary artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("portfolio: close temporary artifact: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("portfolio: commit artifact: %w", err)
	}
	return nil
}

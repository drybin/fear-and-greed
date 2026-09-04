package orchestration_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/orchestration"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/drybin/fear-and-greed/internal/research/validation"
	"github.com/stretchr/testify/require"
)

func TestPrepareManifestFromFrozenCandles(t *testing.T) {
	dir := t.TempDir()
	candleDir := filepath.Join(dir, "candles")
	require.NoError(t, os.MkdirAll(candleDir, 0o755))
	cutoff := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	var symbols strings.Builder
	for i := 0; i < 50; i++ {
		symbol := fmt.Sprintf("C%02dUSDT", i)
		fmt.Fprintln(&symbols, symbol)
		csv := "open_time,open,high,low,close,volume\n" +
			cutoff.Add(-2*time.Minute).Format("2006-01-02 15:04:05") + ",1,2,1,2,1\n" +
			cutoff.Add(-time.Minute).Format("2006-01-02 15:04:05") + ",2,3,2,3,1\n"
		require.NoError(t, os.WriteFile(filepath.Join(candleDir, symbol+".csv"), []byte(csv), 0o644))
	}
	symbolsPath := filepath.Join(dir, "symbols.txt")
	require.NoError(t, os.WriteFile(symbolsPath, []byte(symbols.String()), 0o644))
	manifestPath := filepath.Join(dir, "manifest.json")

	m, err := orchestration.PrepareManifest(orchestration.PrepareManifestOptions{
		SymbolsFile: symbolsPath, CandleDir: candleDir, OutputPath: manifestPath,
		Cutoff: cutoff, Source: manifest.SourceRevision{GitRevision: "abc123"}, Seed: 42,
	})
	require.NoError(t, err)
	require.Len(t, m.Universe.Symbols, 50)
	require.Len(t, m.Strategies, 4)
	folds, err := validation.GenerateFolds(m.Schedule, 0)
	require.NoError(t, err)
	require.Len(t, folds, 3)
	require.Equal(t, cutoff, m.Schedule.LockedHoldout.End)

	raw, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	decoded, err := manifest.Decode(raw)
	require.NoError(t, err)
	require.Equal(t, m.ID, decoded.ID)
	_, err = orchestration.NewInProcessRunner(decoded, orchestration.DirCandleStore{Dir: candleDir})
	require.NoError(t, err)
}

func TestPrepareManifestResearchV3Suite(t *testing.T) {
	dir := t.TempDir()
	candleDir := filepath.Join(dir, "candles")
	require.NoError(t, os.MkdirAll(candleDir, 0o755))
	cutoff := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	var symbols strings.Builder
	for i := 0; i < 50; i++ {
		symbol := fmt.Sprintf("C%02dUSDT", i)
		fmt.Fprintln(&symbols, symbol)
		csv := "open_time,open,high,low,close,volume\n" +
			cutoff.Add(-2*time.Minute).Format("2006-01-02 15:04:05") + ",1,2,1,2,1\n" +
			cutoff.Add(-time.Minute).Format("2006-01-02 15:04:05") + ",2,3,2,3,1\n"
		require.NoError(t, os.WriteFile(filepath.Join(candleDir, symbol+".csv"), []byte(csv), 0o644))
	}
	symbolsPath := filepath.Join(dir, "symbols.txt")
	require.NoError(t, os.WriteFile(symbolsPath, []byte(symbols.String()), 0o644))
	m, err := orchestration.PrepareManifest(orchestration.PrepareManifestOptions{
		SymbolsFile: symbolsPath, CandleDir: candleDir, OutputPath: filepath.Join(dir, "manifest-v3.json"),
		Cutoff: cutoff, Source: manifest.SourceRevision{GitRevision: "abc123"}, Seed: 42, Suite: "research-v3",
	})
	require.NoError(t, err)
	require.Len(t, m.Strategies, 3)
	require.NoError(t, manifest.ValidateResearchV3StrategyCodes(m.Strategies))
}

func TestPrepareManifestDailyLowZoneV11Suite(t *testing.T) {
	dir := t.TempDir()
	candleDir := filepath.Join(dir, "candles")
	require.NoError(t, os.MkdirAll(candleDir, 0o755))
	cutoff := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	symbolsPath := filepath.Join(dir, "symbols.txt")
	var symbols strings.Builder
	for i := 0; i < 50; i++ {
		symbol := fmt.Sprintf("D%02dUSDT", i)
		fmt.Fprintln(&symbols, symbol)
		require.NoError(t, os.WriteFile(filepath.Join(candleDir, symbol+".csv"), []byte("open_time,open,high,low,close,volume\n2026-07-31 23:58:00,1,2,1,2,1\n2026-07-31 23:59:00,2,3,2,3,1\n"), 0o644))
	}
	require.NoError(t, os.WriteFile(symbolsPath, []byte(symbols.String()), 0o644))
	m, err := orchestration.PrepareManifest(orchestration.PrepareManifestOptions{SymbolsFile: symbolsPath, CandleDir: candleDir, OutputPath: filepath.Join(dir, "manifest.json"), Cutoff: cutoff, Source: manifest.SourceRevision{GitRevision: "abc123"}, Seed: 42, Suite: "daily-low-zone-v1_1"})
	require.NoError(t, err)
	require.Len(t, m.Strategies, 1)
	require.Equal(t, protocolv2.StrategyCode("daily-low-zone-v1"), m.Strategies[0].Ref.Code)
	require.Equal(t, protocolv2.StrategyVersion("v1.1.0"), m.Strategies[0].Ref.Version)
}

func TestPrepareManifestDailyLowZoneV12Suite(t *testing.T) {
	dir := t.TempDir()
	candleDir := filepath.Join(dir, "candles")
	require.NoError(t, os.MkdirAll(candleDir, 0o755))
	cutoff := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	symbolsPath := filepath.Join(dir, "symbols.txt")
	var symbols strings.Builder
	for i := 0; i < 50; i++ {
		symbol := fmt.Sprintf("D%02dUSDT", i)
		fmt.Fprintln(&symbols, symbol)
		require.NoError(t, os.WriteFile(filepath.Join(candleDir, symbol+".csv"), []byte("open_time,open,high,low,close,volume\n2026-07-31 23:58:00,1,2,1,2,1\n2026-07-31 23:59:00,2,3,2,3,1\n"), 0o644))
	}
	require.NoError(t, os.WriteFile(symbolsPath, []byte(symbols.String()), 0o644))
	m, err := orchestration.PrepareManifest(orchestration.PrepareManifestOptions{SymbolsFile: symbolsPath, CandleDir: candleDir, OutputPath: filepath.Join(dir, "manifest.json"), Cutoff: cutoff, Source: manifest.SourceRevision{GitRevision: "abc123"}, Seed: 42, Suite: "daily-low-zone-v1_2"})
	require.NoError(t, err)
	require.Len(t, m.Strategies, 1)
	require.Equal(t, protocolv2.StrategyCode("daily-low-zone-v1"), m.Strategies[0].Ref.Code)
	require.Equal(t, protocolv2.StrategyVersion("v1.2.0"), m.Strategies[0].Ref.Version)
}

func TestPrepareManifestDailyLowZoneV13Suite(t *testing.T) {
	dir := t.TempDir()
	candleDir := filepath.Join(dir, "candles")
	require.NoError(t, os.MkdirAll(candleDir, 0o755))
	cutoff := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	symbolsPath := filepath.Join(dir, "symbols.txt")
	var symbols strings.Builder
	for i := 0; i < 50; i++ {
		symbol := fmt.Sprintf("D%02dUSDT", i)
		fmt.Fprintln(&symbols, symbol)
		require.NoError(t, os.WriteFile(filepath.Join(candleDir, symbol+".csv"), []byte("open_time,open,high,low,close,volume\n2026-07-31 23:58:00,1,2,1,2,1\n2026-07-31 23:59:00,2,3,2,3,1\n"), 0o644))
	}
	require.NoError(t, os.WriteFile(symbolsPath, []byte(symbols.String()), 0o644))
	m, err := orchestration.PrepareManifest(orchestration.PrepareManifestOptions{SymbolsFile: symbolsPath, CandleDir: candleDir, OutputPath: filepath.Join(dir, "manifest.json"), Cutoff: cutoff, Source: manifest.SourceRevision{GitRevision: "abc123"}, Seed: 42, Suite: "daily-low-zone-v1_3"})
	require.NoError(t, err)
	require.Len(t, m.Strategies, 1)
	require.Equal(t, protocolv2.StrategyVersion("v1.3.0"), m.Strategies[0].Ref.Version)
}

func TestPrepareManifestDonchianBreakoutV1Suite(t *testing.T) {
	dir := t.TempDir()
	candleDir := filepath.Join(dir, "candles")
	require.NoError(t, os.MkdirAll(candleDir, 0o755))
	cutoff := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	symbolsPath := filepath.Join(dir, "symbols.txt")
	var symbols strings.Builder
	for i := 0; i < 50; i++ {
		symbol := fmt.Sprintf("D%02dUSDT", i)
		fmt.Fprintln(&symbols, symbol)
		require.NoError(t, os.WriteFile(filepath.Join(candleDir, symbol+".csv"), []byte("open_time,open,high,low,close,volume\n2026-07-31 23:58:00,1,2,1,2,1\n2026-07-31 23:59:00,2,3,2,3,1\n"), 0o644))
	}
	require.NoError(t, os.WriteFile(symbolsPath, []byte(symbols.String()), 0o644))
	m, err := orchestration.PrepareManifest(orchestration.PrepareManifestOptions{SymbolsFile: symbolsPath, CandleDir: candleDir, OutputPath: filepath.Join(dir, "manifest.json"), Cutoff: cutoff, Source: manifest.SourceRevision{GitRevision: "abc123"}, Seed: 42, Suite: "donchian-breakout-v1"})
	require.NoError(t, err)
	require.Len(t, m.Strategies, 1)
	require.Equal(t, protocolv2.StrategyCode("donchian-breakout-long-v1"), m.Strategies[0].Ref.Code)
	require.Equal(t, protocolv2.StrategyVersion("v1.0.0"), m.Strategies[0].Ref.Version)
}

func TestPrepareManifestBollingerRangeReversionV1Suite(t *testing.T) {
	dir := t.TempDir()
	candleDir := filepath.Join(dir, "candles")
	require.NoError(t, os.MkdirAll(candleDir, 0o755))
	cutoff := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	symbolsPath := filepath.Join(dir, "symbols.txt")
	var symbols strings.Builder
	for i := 0; i < 50; i++ {
		symbol := fmt.Sprintf("B%02dUSDT", i)
		fmt.Fprintln(&symbols, symbol)
		require.NoError(t, os.WriteFile(filepath.Join(candleDir, symbol+".csv"), []byte("open_time,open,high,low,close,volume\n2026-07-31 23:58:00,1,2,1,2,1\n2026-07-31 23:59:00,2,3,2,3,1\n"), 0o644))
	}
	require.NoError(t, os.WriteFile(symbolsPath, []byte(symbols.String()), 0o644))
	m, err := orchestration.PrepareManifest(orchestration.PrepareManifestOptions{SymbolsFile: symbolsPath, CandleDir: candleDir, OutputPath: filepath.Join(dir, "manifest.json"), Cutoff: cutoff, Source: manifest.SourceRevision{GitRevision: "abc123"}, Seed: 42, Suite: "bollinger-range-reversion-v1"})
	require.NoError(t, err)
	require.Len(t, m.Strategies, 1)
	require.Equal(t, protocolv2.StrategyCode("bollinger-range-reversion-long-v1"), m.Strategies[0].Ref.Code)
	require.Equal(t, protocolv2.StrategyVersion("v1.0.0"), m.Strategies[0].Ref.Version)
}

func TestPrepareManifestCapitulationReversalV1Suite(t *testing.T) {
	dir := t.TempDir()
	candleDir := filepath.Join(dir, "candles")
	require.NoError(t, os.MkdirAll(candleDir, 0o755))
	cutoff := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	symbolsPath := filepath.Join(dir, "symbols.txt")
	var symbols strings.Builder
	for i := 0; i < 50; i++ {
		symbol := fmt.Sprintf("C%02dUSDT", i)
		fmt.Fprintln(&symbols, symbol)
		require.NoError(t, os.WriteFile(filepath.Join(candleDir, symbol+".csv"), []byte("open_time,open,high,low,close,volume\n2026-07-31 23:58:00,1,2,1,2,1\n2026-07-31 23:59:00,2,3,2,3,1\n"), 0o644))
	}
	require.NoError(t, os.WriteFile(symbolsPath, []byte(symbols.String()), 0o644))
	m, err := orchestration.PrepareManifest(orchestration.PrepareManifestOptions{SymbolsFile: symbolsPath, CandleDir: candleDir, OutputPath: filepath.Join(dir, "manifest.json"), Cutoff: cutoff, Source: manifest.SourceRevision{GitRevision: "abc123"}, Seed: 42, Suite: "capitulation-reversal-v1"})
	require.NoError(t, err)
	require.Len(t, m.Strategies, 1)
	require.Equal(t, protocolv2.StrategyCode("capitulation-reversal-long-v1"), m.Strategies[0].Ref.Code)
	require.Equal(t, protocolv2.StrategyVersion("v1.0.0"), m.Strategies[0].Ref.Version)
}

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

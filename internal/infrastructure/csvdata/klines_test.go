package csvdata

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadKlinesReadsRowsIncrementally(t *testing.T) {
	path := filepath.Join(t.TempDir(), "BTCUSDT.csv")
	data := "open_time,open,high,low,close,volume\n" +
		"2025-01-01 00:00:00,100,102,99,101,12.5\n" +
		"2025-01-01 00:01:00,101,103,100,102,13.5\n"
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))

	candles, err := LoadKlines(path)
	require.NoError(t, err)
	require.Len(t, candles, 2)
	require.Equal(t, time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), candles[0].OpenTime)
	require.Equal(t, 100.0, candles[0].Open)
	require.Equal(t, 13.5, candles[1].Volume)
}

func TestLoadKlinesRejectsHeaderOnlyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.csv")
	require.NoError(t, os.WriteFile(path, []byte("open_time,open,high,low,close,volume\n"), 0o600))

	_, err := LoadKlines(path)
	require.ErrorContains(t, err, "csv is empty")
}

func TestLoadKlinesRangeUsesHalfOpenBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "BTCUSDT.csv")
	data := "open_time,open,high,low,close,volume\n" +
		"2025-01-01 00:00:00,100,102,99,101,12.5\n" +
		"2025-01-01 00:01:00,101,103,100,102,13.5\n" +
		"2025-01-01 00:02:00,102,104,101,103,14.5\n"
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))
	start := time.Date(2025, 1, 1, 0, 1, 0, 0, time.UTC)

	candles, err := LoadKlinesRange(path, start, start.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, candles, 1)
	require.Equal(t, start, candles[0].OpenTime)
}

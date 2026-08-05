package eligibility_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/eligibility"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func TestInventoryFileFindsQualityProblemsAndFingerprintsBytes(t *testing.T) {
	path := writeFixture(t, "candles.csv", `open_time,open,high,low,close,volume
2023-01-01 00:00:00,1,2,0.5,1.5,10
2023-01-01 01:00:00,1,2,0.5,1.5,nope
2023-01-01 01:00:00,1,2,0.5,1.5,10
2023-01-01 04:00:00,NaN,2,0.5,1.5,10
bad,1,2,0.5,1.5,10
`)

	inventory, err := eligibility.InventoryFile(path)
	require.NoError(t, err)
	require.Equal(t, 5, inventory.RowCount)
	require.Equal(t, []string{"open_time", "open", "high", "low", "close", "volume"}, inventory.Columns)
	require.Equal(t, time.Hour, inventory.Interval)
	require.Len(t, string(inventory.SHA256), 64)
	require.False(t, inventory.CoreUsable())
	require.Equal(t, 1, inventory.Volume.MalformedRows)
	requireIssue(t, inventory, eligibility.IssueDuplicate)
	requireIssue(t, inventory, eligibility.IssueNonFiniteOHLC)
	requireIssue(t, inventory, eligibility.IssueMalformed)

	require.NoError(t, os.WriteFile(path, []byte("open_time,open,high,low,close\n"), 0o600))
	changed, err := eligibility.InventoryFile(path)
	require.NoError(t, err)
	require.NotEqual(t, inventory.SHA256, changed.SHA256)
}

func TestInventoryFileDetectsUnorderedAndMissingIntervals(t *testing.T) {
	path := writeFixture(t, "candles.csv", `open_time,open,high,low,close
2023-01-01 00:00:00,1,2,0.5,1.5
2023-01-01 02:00:00,1,2,0.5,1.5
2023-01-01 01:00:00,1,2,0.5,1.5
2023-01-01 04:00:00,1,2,0.5,1.5
`)

	inventory, err := eligibility.InventoryFile(path)
	require.NoError(t, err)
	require.Equal(t, time.Hour, inventory.Interval)
	requireIssue(t, inventory, eligibility.IssueUnordered)
	requireIssue(t, inventory, eligibility.IssueMissing)
}

func TestLoadFrozenSnapshotUsesExistingSymbolListFormat(t *testing.T) {
	path := writeFixture(t, "snapshot.txt", "# an existing scripts/symbols_*.txt-style file\nBTCUSDT\nETHUSDT\n")
	asOf := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)

	snapshot, err := eligibility.LoadFrozenSnapshot(path, "cmc-50-200", asOf)
	require.NoError(t, err)
	require.Equal(t, asOf, snapshot.AsOf)
	require.Equal(t, []protocolv2.Symbol{"BTCUSDT", "ETHUSDT"}, snapshot.Symbols)
	require.Len(t, string(snapshot.SHA256), 64)
}

func TestEvaluateCohortSeparatesShortHistoryAndWarmup(t *testing.T) {
	test := protocolv2.TimeRange{
		Start: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
	}
	snapshot := eligibility.FrozenSnapshot{
		Name:    "top-50",
		AsOf:    test.Start,
		Symbols: []protocolv2.Symbol{"LONGUSDT", "SHORTUSDT", "WARMUSDT"},
	}
	inventories := map[protocolv2.Symbol]eligibility.FileInventory{
		"LONGUSDT": inventoryAt(
			time.Date(2022, 12, 31, 0, 0, 0, 0, time.UTC),
			time.Date(2023, 12, 30, 0, 0, 0, 0, time.UTC),
			time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
		),
		"SHORTUSDT": inventoryAt(time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC)),
		"WARMUSDT": inventoryAt(
			time.Date(2022, 12, 31, 0, 0, 0, 0, time.UTC),
			time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
		),
	}

	report, err := eligibility.EvaluateCohort(snapshot, inventories, test, 3)
	require.NoError(t, err)
	require.Equal(t, protocolv2.UniverseFrozenCurrentCohort, report.Provenance)
	require.Equal(t, eligibility.SurvivorshipWarning, report.SurvivorshipWarning)
	require.Len(t, report.Primary, 1)
	require.Equal(t, protocolv2.Symbol("LONGUSDT"), report.Primary[0].Symbol)
	require.Len(t, report.Secondary, 2)
	require.Contains(t, report.Secondary[0].ExclusionReasons, eligibility.ExclusionShortHistory)
	require.Contains(t, report.Secondary[1].ExclusionReasons, eligibility.ExclusionInsufficientWarmup)
}

func inventoryAt(timestamps ...time.Time) eligibility.FileInventory {
	return eligibility.FileInventory{
		RowCount:   len(timestamps),
		Timestamps: timestamps,
		Range: protocolv2.TimeRange{
			Start: timestamps[0],
			End:   timestamps[len(timestamps)-1].Add(24 * time.Hour),
		},
	}
}

func requireIssue(t *testing.T, inventory eligibility.FileInventory, kind eligibility.IssueKind) {
	t.Helper()
	for _, issue := range inventory.Issues {
		if issue.Kind == kind {
			return
		}
	}
	require.Failf(t, "missing issue", "expected %s in %#v", kind, inventory.Issues)
}

func writeFixture(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

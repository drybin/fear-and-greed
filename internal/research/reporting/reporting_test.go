package reporting

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/metrics"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func TestWriteJSONAtomicGolden(t *testing.T) {
	report := SummaryReport{
		ArtifactHeader: Header(SummarySchemaVersion),
		ExperimentID:   "fixture",
		GeneratedAt:    time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Metrics:        metrics.Summary{TradeCount: 0, SymbolWinRate: map[string]*float64{}},
	}
	path := filepath.Join(t.TempDir(), "reports", "summary.json")
	completed, err := WriteJSON(path, report)
	require.NoError(t, err)
	require.NoError(t, completed.SHA256.Validate())
	verified, err := VerifyCompletedArtifact(path)
	require.NoError(t, err)
	require.Equal(t, completed, verified)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.JSONEq(t, `{
		"schema_version":"report.summary.v1",
		"protocol_version":"report.v1",
		"experiment_id":"fixture",
		"generated_at":"2024-01-01T00:00:00Z",
		"metrics":{"gross_return":null,"net_return":null,"annualized_return":null,"drawdown":null,"max_drawdown":0,"max_drawdown_duration_seconds":0,"calmar":null,"expectancy_currency":null,"expectancy_r":null,"profit_factor":null,"payoff_ratio":null,"trade_count":0,"closed_trade_count":0,"wins":0,"losses":0,"breakevens":0,"trade_win_rate":null,"average_holding_seconds":null,"median_holding_seconds":null,"exposure":null,"capital_utilization":null,"turnover":null,"total_commission":0,"total_slippage":0,"symbol_win_rate":{},"breadth":0,"contribution_concentration":null}
	}`, string(data))
}

func TestSchemaValidation(t *testing.T) {
	valid := CandidateReport{
		ArtifactHeader: Header(CandidateSchemaVersion),
		ExperimentID:   "fixture", CandidateID: "candidate-1",
		Strategy:  protocolv2.StrategyRef{Code: "test", Version: "v1"},
		Selection: SelectionExplanation{Rank: 1, Explanation: "highest deterministic score"},
	}
	require.NoError(t, Validate(valid))
	valid.SchemaVersion = FoldSchemaVersion
	require.Error(t, Validate(valid))
}

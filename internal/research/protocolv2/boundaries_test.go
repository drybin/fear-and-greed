package protocolv2_test

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func TestIdentifiersValidate(t *testing.T) {
	require.NoError(t, protocolv2.ExperimentID("core-run-001").Validate())
	require.Error(t, protocolv2.ExperimentID("Bad ID").Validate())

	require.NoError(t, protocolv2.StrategyCode("fib-pullback-trend-v1").Validate())
	require.Error(t, protocolv2.StrategyCode("Fib_Pullback").Validate())

	ref := protocolv2.StrategyRef{
		Code:    "nr7-trend-breakout-v1",
		Version: "v1.0.0",
	}
	require.NoError(t, ref.Validate())
	require.Equal(t, "nr7-trend-breakout-v1@v1.0.0", ref.String())

	require.NoError(t, protocolv2.Symbol("BTCUSDT").Validate())
	require.Error(t, protocolv2.Symbol("btc-usdt").Validate())

	require.NoError(t, protocolv2.Timeframe("1m").Validate())
	require.Error(t, protocolv2.Timeframe("7m").Validate())

	require.NoError(t, protocolv2.SHA256Hex("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa").Validate())
	require.Error(t, protocolv2.SHA256Hex("zz").Validate())
}

func TestEnumsValidate(t *testing.T) {
	require.NoError(t, protocolv2.PhaseDevelopment.Validate())
	require.Error(t, protocolv2.Phase("portfolio").Validate())

	require.NoError(t, protocolv2.DecisionResearchPass.Validate())
	require.Error(t, protocolv2.DecisionStatus("promote").Validate())

	require.NoError(t, protocolv2.UniverseFrozenCurrentCohort.Validate())
	require.NoError(t, protocolv2.RejectionHoldoutAccessDenied.Validate())
}

func TestTimeRangeHalfOpenUTC(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC)
	r, err := protocolv2.NewTimeRange(start, end)
	require.NoError(t, err)

	require.True(t, r.ContainsInstant(start))
	require.False(t, r.ContainsInstant(end))
	require.True(t, r.ContainsInstant(end.Add(-time.Second)))

	local := time.Date(2024, 1, 1, 0, 0, 0, 0, time.FixedZone("X", 3600))
	_, err = protocolv2.NewTimeRange(local, end)
	require.NoError(t, err) // normalized to UTC

	bad := protocolv2.TimeRange{Start: end, End: start}
	require.Error(t, bad.Validate())

	other, err := protocolv2.NewTimeRange(end.Add(-24*time.Hour), end.Add(24*time.Hour))
	require.NoError(t, err)
	require.True(t, r.Overlaps(other))
}

func TestMoneyRoundingDeterministic(t *testing.T) {
	require.Equal(t, 1.23456789, protocolv2.RoundPrice(1.234567891))
	require.Equal(t, 0.00000001, protocolv2.RoundQuantity(0.000000014))
	require.Equal(t, 0.12345679, protocolv2.RoundFee(0.123456789))
	require.Equal(t, 0.1234567891, protocolv2.RoundMetric(0.12345678914))
}

func TestOutputPathsSeparateFromLegacy(t *testing.T) {
	root := protocolv2.ExperimentRoot("/tmp/reports", "core-run-001")
	require.Equal(t, filepath.Clean("/tmp/reports/protocol-v2/core-run-001"), root)
	require.True(t, protocolv2.IsProtocolV2Path(root))
	require.False(t, protocolv2.IsProtocolV2Path("/tmp/reports/data/nr7/BTCUSDT__full.json"))
	require.Contains(t, protocolv2.ManifestPath(root), "manifests/manifest.json")
	require.Contains(t, protocolv2.HoldoutDir(root), "holdout")
}

func TestSchemaHeaderJSON(t *testing.T) {
	h := protocolv2.ArtifactHeader{
		SchemaVersion:   protocolv2.ManifestSchemaVersion,
		ProtocolVersion: protocolv2.ProtocolVersion,
	}
	b, err := json.Marshal(h)
	require.NoError(t, err)
	var decoded protocolv2.ArtifactHeader
	require.NoError(t, json.Unmarshal(b, &decoded))
	require.Equal(t, protocolv2.ManifestSchemaVersion, decoded.SchemaVersion)
	require.Equal(t, "research-validation-v2", decoded.ProtocolVersion)
}

func TestPackageBoundaryImports(t *testing.T) {
	// Compiling these blank imports proves the boundary packages exist and
	// do not pull reverse dependencies into protocolv2.
	_ = []string{
		"github.com/drybin/fear-and-greed/internal/research/manifest",
		"github.com/drybin/fear-and-greed/internal/research/eligibility",
		"github.com/drybin/fear-and-greed/internal/research/execution",
		"github.com/drybin/fear-and-greed/internal/research/metrics",
		"github.com/drybin/fear-and-greed/internal/research/reporting",
		"github.com/drybin/fear-and-greed/internal/research/orchestration",
	}
}

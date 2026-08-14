package candidates

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/drybin/fear-and-greed/internal/strategy"
	"github.com/stretchr/testify/require"
)

func TestCoreRegistryContainsExactlyFourApprovedCandidates(t *testing.T) {
	registry := execution.NewRegistry()
	require.NoError(t, RegisterCore(registry))
	require.Equal(t, []protocolv2.StrategyCode{
		BreakoutRetestLongCode, FibPullbackTrendCode, NR7TrendBreakoutCode, VolatilityCompressionBreakoutCode,
	}, []protocolv2.StrategyCode{
		registry.List()[0].Ref.Code, registry.List()[1].Ref.Code, registry.List()[2].Ref.Code, registry.List()[3].Ref.Code,
	})
}

func TestGridsAreBoundedFrozenAndSymbolIndependent(t *testing.T) {
	for _, candidate := range Core() {
		grid := candidate.Grid()
		require.NotEmpty(t, grid, candidate.Metadata().Ref.String())
		require.LessOrEqual(t, len(grid), 30, candidate.Metadata().Ref.String())
		grid[0].Values["mutated"] = true
		require.NotContains(t, candidate.Grid()[0].Values, "mutated")
		require.Equal(t, candidate.Grid(), candidate.Grid(), "grid must not depend on a symbol or test period")
	}
}

func TestAllAdaptersHaveInsufficientWarmupAndInvalidStopFixtures(t *testing.T) {
	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, candidate := range Core() {
		t.Run(candidate.Metadata().Ref.String(), func(t *testing.T) {
			// Zero-cost legacy fixture: no signal is possible before the declared
			// strategy warmup. This never runs Engine or consults a test period.
			signals, err := candidate.Signals("BTCUSDT", []model.Candle{{OpenTime: t0, Open: 100, High: 101, Low: 99, Close: 100}}, candidate.Grid()[0].ID)
			require.NoError(t, err)
			require.Empty(t, signals)

			// A positive but inverted stop is a valid decision record. The
			// engine must reject it against the next executable open.
			base := candidate.(adapter)
			signal, err := base.signal("BTCUSDT", candidate.Grid()[0].ID, 0, strategy.EntrySignal{
				Time: t0, EntryPrice: 100, Stop: 101, TP1: 110, TP2: 120, Diagnostics: map[string]float64{"fixture": 1},
			})
			require.NoError(t, err)
			engine, err := execution.NewEngine(execution.Config{InitialEquity: 10_000, Interval: time.Hour, CostProfile: "zero"})
			require.NoError(t, err)
			result, err := engine.Run([]execution.Candle{
				{Time: t0, Open: 100, High: 101, Low: 99, Close: 100},
				{Time: t0.Add(time.Hour), Open: 100, High: 101, Low: 99, Close: 100},
			}, []execution.CloseConfirmedSignal{signal})
			require.NoError(t, err)
			require.Len(t, result.Rejections, 1)
			require.Equal(t, protocolv2.RejectionInvalidStop, result.Rejections[0].Reason)
		})
	}
}

func TestLegacyEntryFixturesMapToCloseConfirmedSignals(t *testing.T) {
	t0 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, candidate := range Core() {
		t.Run(candidate.Metadata().Ref.String(), func(t *testing.T) {
			base := candidate.(adapter)
			got, err := base.signal("BTCUSDT", candidate.Grid()[0].ID, 0, strategy.EntrySignal{
				Time: t0, EntryPrice: 100, Stop: 90, TP1: 110, TP2: 120, Diagnostics: map[string]float64{"legacy_fixture": 0},
			})
			require.NoError(t, err)
			require.Equal(t, t0, got.SourceCandleTime)
			require.Equal(t, 90.0, got.Stop)
			require.Equal(t, []execution.Target{{Name: "tp1", Price: 110}, {Name: "tp2", Price: 120}}, got.Targets)
			require.Equal(t, 100.0, got.Diagnostics["legacy_entry_price"])
			require.NoError(t, got.Validate())
		})
	}
}

func TestResearchV3AdaptersHaveBoundedCommonContract(t *testing.T) {
	adapters := ResearchV3()
	require.Len(t, adapters, 3)
	for _, adapter := range adapters {
		metadata := adapter.Metadata()
		require.Greater(t, metadata.WarmupBars, 0)
		require.NoError(t, metadata.Ref.Validate())
	}
	require.Equal(t, protocolv2.Timeframe("15m"), adapters[2].Metadata().Timeframe)
	require.Len(t, adapters[2].Grid(), 1)
}

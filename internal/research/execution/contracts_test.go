package execution_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

var testTime = time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)

func testRef() protocolv2.StrategyRef {
	return protocolv2.StrategyRef{Code: "nr7-trend-breakout-v1", Version: "v1.0.0"}
}

func testMetadata() execution.StrategyMetadata {
	return execution.StrategyMetadata{
		Ref: testRef(), Name: "NR7 Trend Breakout", Timeframe: "1h", WarmupBars: 220,
	}
}

func testSignal() execution.CloseConfirmedSignal {
	return execution.CloseConfirmedSignal{
		SignalID: "sig-001", Strategy: testRef(), Symbol: "BTCUSDT", Timeframe: "1h",
		SourceCandleTime: testTime, Side: execution.SideLong, Stop: 100,
		Targets:     []execution.Target{{Name: "tp1", Price: 120}},
		Diagnostics: map[string]float64{"atr": 4.5},
	}
}

func testIntent() execution.OrderIntent {
	return execution.OrderIntent{
		IntentID: "intent-001", SignalID: "sig-001", Strategy: testRef(), Symbol: "BTCUSDT",
		Side: execution.SideLong, SourceCandleTime: testTime, EligibleAt: testTime.Add(time.Hour),
		Quantity: 2, Stop: 100, Targets: []execution.Target{{Name: "tp1", Price: 120}},
	}
}

func testAudit(id string, quantity float64, fillTime time.Time) execution.FillAudit {
	return execution.FillAudit{
		FillID: id, IntentID: "intent-001", SignalID: "sig-001", Strategy: testRef(),
		Symbol: "BTCUSDT", Side: execution.SideLong, SourceCandleTime: testTime,
		FillTime: fillTime, ReferencePrice: 110, Price: 110.055, Quantity: quantity,
		Commission: 0.22, Slippage: 0.11, CostProfile: "base",
		Audit: map[string]float64{"bar_open": 110},
	}
}

func TestDomainRecordsSerializeAndValidate(t *testing.T) {
	entry := execution.EntryFill{
		FillAudit: testAudit("fill-entry", 2, testTime.Add(time.Hour)), PositionID: "position-001",
	}
	partial := execution.PartialExitFill{
		FillAudit:  testAudit("fill-partial", 1, testTime.Add(2*time.Hour)),
		PositionID: "position-001", Reason: execution.ExitReasonTarget,
	}
	final := execution.FinalExitFill{
		FillAudit:  testAudit("fill-final", 1, testTime.Add(3*time.Hour)),
		PositionID: "position-001", Reason: execution.ExitReasonStop,
	}
	trade := execution.TradeState{
		TradeID: "trade-001", PositionID: "position-001", Status: execution.TradeClosed,
		Entry: entry, PartialExits: []execution.PartialExitFill{partial}, FinalExit: &final,
	}

	assertJSONRoundTrip(t, testMetadata())
	assertJSONRoundTrip(t, execution.Target{Name: "tp1", Price: 120})
	assertJSONRoundTrip(t, testSignal())
	assertJSONRoundTrip(t, testIntent())
	assertJSONRoundTrip(t, entry)
	assertJSONRoundTrip(t, partial)
	assertJSONRoundTrip(t, final)
	assertJSONRoundTrip(t, execution.PositionState{
		PositionID: "position-001", Strategy: testRef(), Symbol: "BTCUSDT",
		Side: execution.SideLong, Status: execution.PositionOpen, OpenedAt: testTime.Add(time.Hour),
		InitialQuantity: 2, RemainingQuantity: 1, AverageEntryPrice: 110.055, Stop: 100,
	})
	assertJSONRoundTrip(t, trade)
	assertJSONRoundTrip(t, execution.EquitySnapshot{
		Time: testTime.Add(time.Hour), Cash: 9780, OpenPositionValue: 110.055,
		RealizedPnL: 0, UnrealizedPnL: 0.055, CommissionCosts: 0.22,
		SlippageCosts: 0.11, TotalEquity: 9890.055,
	})
	assertJSONRoundTrip(t, execution.SignalRejection{
		SignalID: "sig-002", Strategy: testRef(), Symbol: "BTCUSDT", OccurredAt: testTime,
		Reason: protocolv2.RejectionInvalidStop, Diagnostics: map[string]float64{"stop": 0},
	})
}

func assertJSONRoundTrip[T interface{ Validate() error }](t *testing.T, record T) {
	t.Helper()
	require.NoError(t, record.Validate())
	encoded, err := json.Marshal(record)
	require.NoError(t, err)
	var decoded T
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, record, decoded)
	require.NoError(t, decoded.Validate())
}

func TestDomainRecordInvariants(t *testing.T) {
	t.Run("intent requires a future eligible time", func(t *testing.T) {
		intent := testIntent()
		intent.EligibleAt = intent.SourceCandleTime
		require.Error(t, intent.Validate())
	})

	t.Run("fill must follow its source candle", func(t *testing.T) {
		fill := testAudit("fill-001", 1, testTime)
		require.Error(t, fill.Validate())
	})

	t.Run("closed position has no remaining quantity", func(t *testing.T) {
		position := execution.PositionState{
			PositionID: "position-001", Strategy: testRef(), Symbol: "BTCUSDT",
			Side: execution.SideLong, Status: execution.PositionClosed, OpenedAt: testTime,
			InitialQuantity: 1, RemainingQuantity: 1, AverageEntryPrice: 100, Stop: 90,
		}
		require.Error(t, position.Validate())
	})

	t.Run("trade cannot sell more than it bought", func(t *testing.T) {
		entry := execution.EntryFill{FillAudit: testAudit("entry", 1, testTime.Add(time.Hour)), PositionID: "position-001"}
		final := execution.FinalExitFill{
			FillAudit:  testAudit("final", 1, testTime.Add(2*time.Hour)),
			PositionID: "position-001", Reason: execution.ExitReasonStop,
		}
		partial := execution.PartialExitFill{
			FillAudit:  testAudit("partial", 0.1, testTime.Add(90*time.Minute)),
			PositionID: "position-001", Reason: execution.ExitReasonTarget,
		}
		trade := execution.TradeState{
			TradeID: "trade-001", PositionID: "position-001", Status: execution.TradeClosed,
			Entry: entry, PartialExits: []execution.PartialExitFill{partial}, FinalExit: &final,
		}
		require.Error(t, trade.Validate())
	})

	t.Run("closed trade reconciles rounded partial and final quantities", func(t *testing.T) {
		entry := execution.EntryFill{
			FillAudit:  testAudit("entry", 20.07533043, testTime.Add(time.Hour)),
			PositionID: "position-001",
		}
		partial := execution.PartialExitFill{
			FillAudit:  testAudit("partial", 10.03766522, testTime.Add(90*time.Minute)),
			PositionID: "position-001", Reason: execution.ExitReasonTarget,
		}
		final := execution.FinalExitFill{
			FillAudit:  testAudit("final", 10.03766521, testTime.Add(2*time.Hour)),
			PositionID: "position-001", Reason: execution.ExitReasonFoldEnd,
		}
		trade := execution.TradeState{
			TradeID: "trade-001", PositionID: "position-001", Status: execution.TradeClosed,
			Entry: entry, PartialExits: []execution.PartialExitFill{partial}, FinalExit: &final,
		}

		require.NoError(t, trade.Validate())
	})

	t.Run("equity reconciles cash and open value", func(t *testing.T) {
		snapshot := execution.EquitySnapshot{
			Time: testTime, Cash: 100, OpenPositionValue: 20, TotalEquity: 121,
		}
		require.Error(t, snapshot.Validate())
	})

	t.Run("rejection reason is protocol defined", func(t *testing.T) {
		rejection := execution.SignalRejection{
			SignalID: "sig-001", Strategy: testRef(), Symbol: "BTCUSDT", OccurredAt: testTime,
			Reason: "nope",
		}
		require.Error(t, rejection.Validate())
	})
}

type testStrategy struct{ metadata execution.StrategyMetadata }

func (s testStrategy) Metadata() execution.StrategyMetadata { return s.metadata }

func TestRegistryRejectsDuplicatesAndListsDeterministically(t *testing.T) {
	registry := execution.NewRegistry()
	strategies := []testStrategy{
		{metadata: execution.StrategyMetadata{Ref: protocolv2.StrategyRef{Code: "zeta-v1", Version: "v1"}, Name: "Zeta", Timeframe: "1h"}},
		{metadata: execution.StrategyMetadata{Ref: protocolv2.StrategyRef{Code: "alpha-v1", Version: "v2"}, Name: "Alpha v2", Timeframe: "1h"}},
		{metadata: execution.StrategyMetadata{Ref: protocolv2.StrategyRef{Code: "alpha-v1", Version: "v1"}, Name: "Alpha v1", Timeframe: "1h"}},
	}
	for _, strategy := range strategies {
		require.NoError(t, registry.Register(strategy))
	}

	err := registry.Register(strategies[0])
	require.Error(t, err)
	require.True(t, errors.Is(err, execution.ErrDuplicateStrategy))

	got := registry.List()
	require.Equal(t, []protocolv2.StrategyRef{
		{Code: "alpha-v1", Version: "v1"},
		{Code: "alpha-v1", Version: "v2"},
		{Code: "zeta-v1", Version: "v1"},
	}, []protocolv2.StrategyRef{got[0].Ref, got[1].Ref, got[2].Ref})
	got[0].Name = "mutated"
	require.Equal(t, "Alpha v1", registry.List()[0].Name)
}

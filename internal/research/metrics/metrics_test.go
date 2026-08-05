package metrics

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func TestCalculateEdgeFixtures(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		in    Input
		check func(t *testing.T, got Summary)
	}{
		{"no trades", Input{InitialEquity: 100, Equity: snapshots(base, 100, 100)}, func(t *testing.T, got Summary) {
			require.Equal(t, 0, got.TradeCount)
			require.Nil(t, got.ProfitFactor)
			require.NotNil(t, got.NetReturn)
		}},
		{"all wins", Input{InitialEquity: 100, Equity: snapshots(base, 100, 120), Trades: []execution.TradeState{trade(base, "one", 110), trade(base.Add(time.Hour), "two", 110)}}, func(t *testing.T, got Summary) {
			require.Equal(t, 2, got.Wins)
			require.Equal(t, 0, got.Losses)
			require.Nil(t, got.ProfitFactor)
		}},
		{"all losses", Input{InitialEquity: 100, Equity: snapshots(base, 100, 80), Trades: []execution.TradeState{trade(base, "one", 90)}}, func(t *testing.T, got Summary) {
			require.Equal(t, 1, got.Losses)
			require.Equal(t, float64(0), *got.ProfitFactor)
			require.Nil(t, got.PayoffRatio)
		}},
		{"breakeven", Input{InitialEquity: 100, Equity: snapshots(base, 100, 100), Trades: []execution.TradeState{trade(base, "one", 100)}}, func(t *testing.T, got Summary) {
			require.Equal(t, 1, got.Breakevens)
			require.Equal(t, float64(0), *got.TradeWinRate)
		}},
		{"open position", Input{InitialEquity: 100, Equity: snapshots(base, 100, 105), Trades: []execution.TradeState{openTrade(base)}}, func(t *testing.T, got Summary) {
			require.Equal(t, 1, got.TradeCount)
			require.Equal(t, 0, got.ClosedTradeCount)
			require.Nil(t, got.ExpectancyCurrency)
		}},
		{"unequal histories", Input{InitialEquity: 100, Equity: []execution.EquitySnapshot{
			snapshot(base, 100), snapshot(base.Add(3*time.Hour), 105), snapshot(base.Add(5*24*time.Hour), 110),
		}}, func(t *testing.T, got Summary) {
			require.NotNil(t, got.AnnualizedReturn)
			require.NotNil(t, got.Exposure)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Calculate(tt.in)
			require.NoError(t, err)
			tt.check(t, got)
		})
	}
}

func TestFoldConsistency(t *testing.T) {
	require.Equal(t, 0.5, *FoldConsistency([]Summary{{NetReturn: floatp(.1)}, {NetReturn: floatp(-.1)}}))
}

func snapshots(at time.Time, values ...float64) []execution.EquitySnapshot {
	out := make([]execution.EquitySnapshot, len(values))
	for i, value := range values {
		out[i] = snapshot(at.Add(time.Duration(i)*24*time.Hour), value)
	}
	return out
}
func snapshot(at time.Time, equity float64) execution.EquitySnapshot {
	return execution.EquitySnapshot{Time: at, Cash: equity, TotalEquity: equity}
}
func trade(at time.Time, id string, exit float64) execution.TradeState {
	entry := fill(at, id, 100)
	end := execution.FinalExitFill{PositionID: "p-" + id, Reason: execution.ExitReasonTarget, FillAudit: fill(at.Add(time.Hour), id+"x", exit)}
	return execution.TradeState{TradeID: id, PositionID: "p-" + id, Status: execution.TradeClosed, Entry: execution.EntryFill{PositionID: "p-" + id, FillAudit: entry}, FinalExit: &end}
}
func openTrade(at time.Time) execution.TradeState {
	id := "open"
	entry := fill(at, id, 100)
	return execution.TradeState{TradeID: id, PositionID: "p-" + id, Status: execution.TradeOpen, Entry: execution.EntryFill{PositionID: "p-" + id, FillAudit: entry}}
}
func fill(at time.Time, id string, price float64) execution.FillAudit {
	return execution.FillAudit{FillID: id, IntentID: "i-" + id, SignalID: "s-" + id, Strategy: protocolv2.StrategyRef{Code: "test", Version: "v1"}, Symbol: "BTCUSDT", Side: execution.SideLong, SourceCandleTime: at.Add(-time.Hour), FillTime: at, ReferencePrice: price, Price: price, Quantity: 1, CostProfile: "base"}
}
func floatp(v float64) *float64 { return &v }

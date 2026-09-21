package metrics

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func TestCalculateShortTradePnL(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ref := protocolv2.StrategyRef{Code: "test", Version: "v1"}
	entry := execution.FillAudit{FillID: "entry", IntentID: "entry", SignalID: "short", Strategy: ref, Symbol: "BTCUSDT", Side: execution.SideShort, SourceCandleTime: start, FillTime: start.Add(time.Hour), ReferencePrice: 100, Price: 100, Quantity: 1, CostProfile: "base"}
	exit := execution.FinalExitFill{PositionID: "position", Reason: execution.ExitReasonTarget, FillAudit: execution.FillAudit{FillID: "exit", IntentID: "exit", SignalID: "short", Strategy: ref, Symbol: "BTCUSDT", Side: execution.SideShort, SourceCandleTime: start, FillTime: start.Add(2 * time.Hour), ReferencePrice: 90, Price: 90, Quantity: 1, CostProfile: "base"}}
	trade := execution.TradeState{TradeID: "trade", PositionID: "position", Status: execution.TradeClosed, Entry: execution.EntryFill{PositionID: "position", FillAudit: entry}, FinalExit: &exit}
	summary, err := Calculate(Input{InitialEquity: 100, Equity: []execution.EquitySnapshot{{Time: start, Cash: 100, TotalEquity: 100}, {Time: start.Add(2 * time.Hour), Cash: 110, TotalEquity: 110}}, Trades: []execution.TradeState{trade}})
	require.NoError(t, err)
	require.Equal(t, 1, summary.Wins)
}

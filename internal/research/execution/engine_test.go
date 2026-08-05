package execution_test

import (
	"math"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func engineConfig() execution.Config {
	return execution.Config{
		InitialEquity: 10_000, Interval: time.Hour, CommissionBPS: 10, SlippageBPS: 5,
		RiskPerTradePercent: 1, MaxNotionalPercent: 20, CostProfile: "base",
	}
}

func engineSignal(id string, source time.Time, stop float64, targets ...execution.Target) execution.CloseConfirmedSignal {
	return execution.CloseConfirmedSignal{
		SignalID: id, Strategy: testRef(), Symbol: "BTCUSDT", Timeframe: "1h",
		SourceCandleTime: source, Side: execution.SideLong, Stop: stop, Targets: targets,
	}
}

func bar(t time.Time, open, high, low, close float64) execution.Candle {
	return execution.Candle{Time: t, Open: open, High: high, Low: low, Close: close}
}

func run(t *testing.T, config execution.Config, candles []execution.Candle, signals ...execution.CloseConfirmedSignal) execution.Result {
	t.Helper()
	engine, err := execution.NewEngine(config)
	require.NoError(t, err)
	result, err := engine.Run(candles, signals)
	require.NoError(t, err)
	return result
}

func TestEngineCausalCostsAndStopFirst(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	result := run(t, engineConfig(), []execution.Candle{
		bar(start, 100, 101, 99, 100),
		bar(start.Add(time.Hour), 100, 120, 89, 110), // stop and target both touched
	}, engineSignal("causal", start, 90, execution.Target{Name: "tp1", Price: 110}))

	require.Len(t, result.Trades, 1)
	trade := result.Trades[0]
	require.Equal(t, start.Add(time.Hour), trade.Entry.FillTime)
	require.Equal(t, execution.ExitReasonStop, trade.FinalExit.Reason)
	require.Greater(t, trade.Entry.Price, trade.Entry.ReferencePrice)
	require.Less(t, trade.FinalExit.Price, trade.FinalExit.ReferencePrice)
	require.Greater(t, trade.Entry.Commission, 0.0)
	require.Greater(t, trade.FinalExit.Commission, 0.0)
	require.Equal(t, trade.Entry.Quantity, trade.FinalExit.Quantity)
}

func TestEngineWaitsForAggregateSignalCandleToClose(t *testing.T) {
	start := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	config := engineConfig()
	result := run(t, config, []execution.Candle{
		bar(start, 100, 101, 99, 100),
		bar(start.Add(time.Minute), 999, 1000, 998, 999), // still inside the signal hour
		bar(start.Add(time.Hour), 101, 102, 100, 101),
	}, engineSignal("hour-close", start, 90))

	require.Len(t, result.Trades, 1)
	require.Equal(t, start.Add(time.Hour), result.Trades[0].Entry.FillTime)
	require.NotEqual(t, 999.0, result.Trades[0].Entry.ReferencePrice)
}

func TestEngineGapPolicyAndGapThroughStop(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []execution.Candle{
		bar(start, 100, 101, 99, 100),
		bar(start.Add(2*time.Hour), 80, 85, 75, 82),
	}
	signal := engineSignal("gap", start, 70)

	rejected := engineConfig()
	rejected.GapPolicy = execution.GapPolicyReject
	result := run(t, rejected, candles, signal)
	require.Empty(t, result.Trades)
	require.Len(t, result.Rejections, 1)
	require.Equal(t, protocolv2.RejectionMissingNextBar, result.Rejections[0].Reason)

	result = run(t, engineConfig(), candles, signal)
	require.Len(t, result.Trades, 1)
	require.Equal(t, candles[1].Time, result.Trades[0].Entry.FillTime)

	stopResult := run(t, engineConfig(), []execution.Candle{
		bar(start, 100, 101, 99, 100),
		bar(start.Add(time.Hour), 100, 105, 95, 101),
		bar(start.Add(2*time.Hour), 80, 85, 75, 82),
	}, engineSignal("gap-stop", start, 90))
	require.Equal(t, execution.ExitReasonStop, stopResult.Trades[0].FinalExit.Reason)
	require.Equal(t, 80.0, stopResult.Trades[0].FinalExit.ReferencePrice)
}

func TestEngineTP1BreakevenTimeAndFoldEndPolicies(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	signal := engineSignal("partial", start, 90, execution.Target{Name: "tp1", Price: 110}, execution.Target{Name: "tp2", Price: 120})
	result := run(t, engineConfig(), []execution.Candle{
		bar(start, 100, 101, 99, 100),
		bar(start.Add(time.Hour), 100, 111, 99, 105),
		bar(start.Add(2*time.Hour), 105, 106, 99, 100), // remainder exits at breakeven
	}, signal)
	require.Len(t, result.Trades, 1)
	require.Len(t, result.Trades[0].PartialExits, 1)
	require.Equal(t, execution.ExitReasonBreakeven, result.Trades[0].FinalExit.Reason)

	open := run(t, engineConfig(), []execution.Candle{
		bar(start, 100, 101, 99, 100),
		bar(start.Add(time.Hour), 100, 105, 99, 102),
	}, engineSignal("open", start, 90))
	require.Len(t, open.Positions, 1)
	require.Equal(t, open.Equity[len(open.Equity)-1].Cash+open.Equity[len(open.Equity)-1].OpenPositionValue, open.Equity[len(open.Equity)-1].TotalEquity)

	closeConfig := engineConfig()
	closeConfig.CloseAtFoldEnd = true
	closed := run(t, closeConfig, []execution.Candle{
		bar(start, 100, 101, 99, 100),
		bar(start.Add(time.Hour), 100, 105, 99, 102),
	}, engineSignal("fold-end", start, 90))
	require.Equal(t, execution.ExitReasonFoldEnd, closed.Trades[0].FinalExit.Reason)
	require.Len(t, closed.Equity, 2)
	require.Equal(t, start.Add(time.Hour), closed.Equity[len(closed.Equity)-1].Time)

	timeConfig := engineConfig()
	timeConfig.TimeExitBars = 1
	timed := run(t, timeConfig, []execution.Candle{
		bar(start, 100, 101, 99, 100),
		bar(start.Add(time.Hour), 100, 105, 99, 102),
		bar(start.Add(2*time.Hour), 102, 103, 101, 102),
	}, engineSignal("time", start, 90))
	require.Equal(t, execution.ExitReasonTime, timed.Trades[0].FinalExit.Reason)
}

func TestEngineSizingInvalidStopAndReconciliation(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	result := run(t, engineConfig(), []execution.Candle{
		bar(start, 100, 101, 99, 100),
		bar(start.Add(time.Hour), 100, 101, 99, 100),
	}, engineSignal("invalid", start, 101))
	require.Len(t, result.Rejections, 1)
	require.Equal(t, protocolv2.RejectionInvalidStop, result.Rejections[0].Reason)

	result = run(t, engineConfig(), []execution.Candle{
		bar(start, 100, 101, 99, 100),
		bar(start.Add(time.Hour), 100, 111, 99, 110),
	}, engineSignal("cap", start, 1, execution.Target{Name: "tp1", Price: 110}))
	require.Len(t, result.Trades, 1)
	require.LessOrEqual(t, result.Trades[0].Entry.Quantity*result.Trades[0].Entry.Price, 2_000.00001)
	for _, snapshot := range result.Equity {
		require.Equal(t, protocolv2.RoundFee(snapshot.Cash+snapshot.OpenPositionValue), snapshot.TotalEquity)
		require.NoError(t, snapshot.Validate())
	}
}

func TestEngineSoldNeverExceedsBoughtAndNetReconciles(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 25; i++ {
		target := 101.0 + float64(i)
		result := run(t, engineConfig(), []execution.Candle{
			bar(start, 100, 101, 99, 100),
			bar(start.Add(time.Hour), 100, target+1, 95, target),
			bar(start.Add(2*time.Hour), target, target+1, 89, 90),
		}, engineSignal("property-"+string(rune('a'+i)), start, 90, execution.Target{Name: "tp1", Price: target}))
		require.Len(t, result.Trades, 1)
		trade := result.Trades[0]
		sold := trade.FinalExit.Quantity
		for _, partial := range trade.PartialExits {
			sold += partial.Quantity
		}
		require.LessOrEqual(t, sold, trade.Entry.Quantity)
		require.NoError(t, trade.Validate())
	}

	result := run(t, engineConfig(), []execution.Candle{
		bar(start, 100, 101, 99, 100),
		bar(start.Add(time.Hour), 100, 111, 89, 90),
	}, engineSignal("net", start, 90))
	trade := result.Trades[0]
	gross := (trade.FinalExit.Price - trade.Entry.Price) * trade.Entry.Quantity
	costs := trade.Entry.Commission + trade.FinalExit.Commission
	net := result.Equity[len(result.Equity)-1].TotalEquity - 10_000
	require.InDelta(t, gross-costs, net, 0.00000002)
	require.False(t, math.IsNaN(net))
}

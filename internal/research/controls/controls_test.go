package controls

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func TestCashAndBuyAndHoldUseCompatibleEngineAccounting(t *testing.T) {
	input := fixtureInput(6)
	config := fixtureConfig()

	cash, err := Cash(config, input)
	require.NoError(t, err)
	require.Empty(t, cash.Engine.Trades)
	require.Len(t, cash.Engine.Equity, len(input.Candles))
	require.Equal(t, protocolv2.RoundFee(config.InitialEquity), cash.Engine.Equity[len(cash.Engine.Equity)-1].TotalEquity)

	config.CloseAtFoldEnd = true
	hold, err := BuyAndHold(config, input)
	require.NoError(t, err)
	require.Equal(t, ref(BuyAndHoldCode), hold.Control)
	require.Len(t, hold.Engine.Trades, 1)
	require.Equal(t, input.Candles[1].Time, hold.Engine.Trades[0].Entry.FillTime)
	require.Equal(t, execution.ExitReasonFoldEnd, hold.Engine.Trades[0].FinalExit.Reason)
	require.Greater(t, hold.Engine.Trades[0].Entry.Commission, 0.0)
	require.Greater(t, hold.Engine.Trades[0].FinalExit.Commission, 0.0)
}

func TestBTCBuyAndHoldUsesExplicitBTCInput(t *testing.T) {
	btc := fixtureInput(5)
	btc.Symbol = "BTCUSDT"
	result, err := BTCBuyAndHold(fixtureConfig(), btc)
	require.NoError(t, err)
	require.Equal(t, BTCBuyAndHoldCode, result.Control.Code)
	require.Equal(t, btc.Symbol, result.Entries[0].Symbol)
}

func TestCausalProtectiveStopRemainsPositiveForLowPricedAssets(t *testing.T) {
	require.Equal(t, 0.00000001, causalProtectiveStop(0.00001))
	require.Equal(t, 0.0001, causalProtectiveStop(100))
}

func TestEMA200GeneratesCausalLongCashSignals(t *testing.T) {
	input := fixtureInput(205)
	for i := range input.Candles {
		price := 100.0
		if i >= 199 && i < 202 {
			price = 150
		}
		if i >= 202 {
			price = 50
		}
		input.Candles[i] = candle(input.Candles[i].Time, price)
	}
	config := fixtureConfig()
	config.CloseAtFoldEnd = true
	result, err := EMA200(config, input)
	require.NoError(t, err)
	require.NotEmpty(t, result.Entries)
	require.NotEmpty(t, result.Exits)
	require.Len(t, result.Engine.Trades, 1)
	require.Equal(t, execution.ExitReasonSignal, result.Engine.Trades[0].FinalExit.Reason)
	require.True(t, result.Engine.Trades[0].FinalExit.FillTime.After(result.Exits[0].SourceCandleTime))
}

func TestRandomEntriesAreSeededAndFrequencyMatched(t *testing.T) {
	input := fixtureInput(12)
	reference := []execution.CloseConfirmedSignal{
		signal(input, 1, "reference-1"),
		signal(input, 4, "reference-2"),
		signal(input, 8, "reference-3"),
	}
	activity, err := MatchActivity(input.Candles, reference)
	require.NoError(t, err)
	require.Equal(t, 3, activity.EntryCount)
	require.Equal(t, 11, activity.HorizonBars)

	first, err := RandomEntries(input, activity, 42)
	require.NoError(t, err)
	second, err := RandomEntries(input, activity, 42)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Len(t, first, activity.EntryCount)
	for _, entry := range first {
		require.Equal(t, RandomCode, entry.Strategy.Code)
		require.True(t, entry.SourceCandleTime.Before(input.Candles[len(input.Candles)-1].Time))
	}
}

func fixtureConfig() execution.Config {
	return execution.Config{
		InitialEquity: 10_000, Interval: time.Hour, CommissionBPS: 10, SlippageBPS: 5,
		RiskPerTradePercent: 1, MaxNotionalPercent: 20, CostProfile: "base",
	}
}

func fixtureInput(count int) Input {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]execution.Candle, count)
	for i := range candles {
		candles[i] = candle(start.Add(time.Duration(i)*time.Hour), 100+float64(i))
	}
	return Input{Symbol: "ETHUSDT", Timeframe: "1h", Candles: candles}
}

func candle(t time.Time, price float64) execution.Candle {
	return execution.Candle{Time: t, Open: price, High: price + 1, Low: price - 1, Close: price}
}

func signal(input Input, index int, id string) execution.CloseConfirmedSignal {
	return execution.CloseConfirmedSignal{
		SignalID: id, Strategy: ref(BuyAndHoldCode), Symbol: input.Symbol, Timeframe: input.Timeframe,
		SourceCandleTime: input.Candles[index].Time, Side: execution.SideLong, Stop: 1,
	}
}

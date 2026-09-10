package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestSpotFlowPullbackV1Signals_UsesOnlyCompletedTrendAndPriorFlow(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	hours := make([]model.Candle, 900)
	for i := range hours {
		price := 100.0 + float64(i)*0.1
		hours[i] = flowHour(start.Add(time.Duration(i)*time.Hour), price, price+0.2, price-0.2, price+0.1, 100)
	}
	// A three-hour pullback followed by the single qualifying flow recovery.
	hours[894] = flowHour(hours[894].OpenTime, 190, 190.2, 189.7, 189.8, 100)
	hours[895] = flowHour(hours[895].OpenTime, 189.8, 190, 188.7, 188.8, 100)
	hours[896] = flowHour(hours[896].OpenTime, 188.8, 189, 187.7, 187.8, 100)
	hours[897] = flowHour(hours[897].OpenTime, 187.8, 191, 187.5, 190.5, 130)

	signals, err := spotFlowPullbackV1Signals(hours, AggregateMinutes(hours, 240))
	require.NoError(t, err)
	require.Len(t, signals, 1)
	require.Equal(t, hours[897].OpenTime, signals[0].Time)
	require.Equal(t, 187.5, signals[0].Stop)

	changed := append([]model.Candle(nil), hours...)
	changed[898].Close = 1 // Future price must not affect the earlier decision.
	after, err := spotFlowPullbackV1Signals(changed, AggregateMinutes(changed, 240))
	require.NoError(t, err)
	require.Equal(t, signals, after)
}

func TestSpotFlowPullbackV1Signals_RejectsMissingFlow(t *testing.T) {
	_, err := SpotFlowPullbackV1Signals([]model.Candle{{OpenTime: time.Now().UTC(), Open: 1, High: 2, Low: 1, Close: 2, Volume: 1}})
	require.ErrorContains(t, err, "missing or invalid quote/trade flow")
}

func flowHour(at time.Time, open, high, low, close, quote float64) model.Candle {
	return model.Candle{
		OpenTime: at, Open: open, High: high, Low: low, Close: close, Volume: quote / close,
		QuoteVolume: quote, Trades: 10, TakerBuyBaseVolume: quote * .6 / close, TakerBuyQuoteVolume: quote * .6,
	}
}

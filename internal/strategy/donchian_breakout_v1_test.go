package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestDonchianBreakoutV1SignalsCausalBreakout(t *testing.T) {
	candles := makeDonchianCandles(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 280)
	signals, err := donchianBreakoutV1Signals(candles, DonchianBreakoutV1Params{ChannelBars: 20, StopATR: 1.5})
	require.NoError(t, err)
	require.Len(t, signals, 1)
	require.Equal(t, candles[260].OpenTime, signals[0].Time)
	require.Greater(t, signals[0].EntryPrice, signals[0].Diagnostics["channel_high"])
	require.Greater(t, signals[0].TP2, signals[0].TP1)
	require.Equal(t, candles[260].OpenTime.Add(DonchianBreakoutTimeExit), signals[0].TimeExitAt)
}

func TestDonchianBreakoutV1RejectsInvalidParametersAndShortHistory(t *testing.T) {
	_, err := DonchianBreakoutV1Signals(nil, DonchianBreakoutV1Params{ChannelBars: 1, StopATR: 1.5})
	require.Error(t, err)
	signals, err := donchianBreakoutV1Signals(makeDonchianCandles(time.Now().UTC(), 200), DonchianBreakoutV1Params{ChannelBars: 20, StopATR: 1.5})
	require.NoError(t, err)
	require.Empty(t, signals)
}

func TestDonchianBreakoutV1NeverUsesFutureBars(t *testing.T) {
	candles := makeDonchianCandles(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 280)
	params := DonchianBreakoutV1Params{ChannelBars: 20, StopATR: 1.5}
	before, err := donchianBreakoutV1Signals(candles, params)
	require.NoError(t, err)

	changed := append([]model.Candle(nil), candles...)
	for i := 270; i < len(changed); i++ {
		changed[i].High *= 10
		changed[i].Close *= 10
	}
	after, err := donchianBreakoutV1Signals(changed, params)
	require.NoError(t, err)
	require.Equal(t, signalsBefore(before, changed[270].OpenTime), signalsBefore(after, changed[270].OpenTime))
}

func makeDonchianCandles(start time.Time, count int) []model.Candle {
	result := make([]model.Candle, count)
	price := 100.0
	for i := range result {
		if i < 260 {
			price += .1
		} else if i == 260 {
			price += 4
		} else {
			price += .05
		}
		result[i] = model.Candle{OpenTime: start.Add(time.Duration(i) * 4 * time.Hour), Open: price - .2, High: price + .5, Low: price - .5, Close: price, Volume: 10}
	}
	return result
}

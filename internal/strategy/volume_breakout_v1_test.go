package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestVolumeBreakoutV1SignalsCausalBreakout(t *testing.T) {
	candles := makeVolumeBreakoutCandles(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 70)
	signals, err := volumeBreakoutV1Signals(candles, VolumeBreakoutV1Params{ChannelBars: 20, VolumeMultiplier: 1.5})
	require.NoError(t, err)
	require.Len(t, signals, 1)
	require.Equal(t, candles[45].OpenTime, signals[0].Time)
	require.Greater(t, signals[0].EntryPrice, signals[0].Diagnostics["prior_high"])
	require.Greater(t, signals[0].Diagnostics["volume"], signals[0].Diagnostics["volume_sma20_prior"])
	require.Greater(t, signals[0].TP2, signals[0].TP1)
	require.Equal(t, candles[45].OpenTime.Add(VolumeBreakoutTimeExit), signals[0].TimeExitAt)
}

func TestVolumeBreakoutV1RejectsInsufficientVolumeAndInvalidParameters(t *testing.T) {
	candles := makeVolumeBreakoutCandles(time.Now().UTC(), 70)
	candles[45].Volume = 10
	signals, err := volumeBreakoutV1Signals(candles, VolumeBreakoutV1Params{ChannelBars: 20, VolumeMultiplier: 1.5})
	require.NoError(t, err)
	require.Empty(t, signals)
	_, err = VolumeBreakoutV1Signals(nil, VolumeBreakoutV1Params{ChannelBars: 10, VolumeMultiplier: 1.5})
	require.Error(t, err)
}

func TestVolumeBreakoutV1NeverUsesFutureBars(t *testing.T) {
	candles := makeVolumeBreakoutCandles(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 70)
	params := VolumeBreakoutV1Params{ChannelBars: 20, VolumeMultiplier: 1.5}
	before, err := volumeBreakoutV1Signals(candles, params)
	require.NoError(t, err)

	changed := append([]model.Candle(nil), candles...)
	for i := 55; i < len(changed); i++ {
		changed[i].High *= 10
		changed[i].Close *= 10
		changed[i].Volume *= 10
	}
	after, err := volumeBreakoutV1Signals(changed, params)
	require.NoError(t, err)
	require.Equal(t, signalsBefore(before, changed[55].OpenTime), signalsBefore(after, changed[55].OpenTime))
}

func makeVolumeBreakoutCandles(start time.Time, count int) []model.Candle {
	result := make([]model.Candle, count)
	for i := range result {
		result[i] = model.Candle{OpenTime: start.Add(time.Duration(i) * time.Hour), Open: 99.7, High: 100, Low: 99.3, Close: 99.8, Volume: 10}
	}
	result[45] = model.Candle{OpenTime: result[45].OpenTime, Open: 99.8, High: 101.2, Low: 99.6, Close: 101, Volume: 20}
	return result
}

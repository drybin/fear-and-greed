package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestCapitulationReversalV1SignalsLaterRecovery(t *testing.T) {
	candles := makeCapitulationCandles(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 50)
	signals, err := capitulationReversalV1Signals(candles, CapitulationReversalV1Params{ReturnPercent: 4, VolumeMultiplier: 2})
	require.NoError(t, err)
	require.Len(t, signals, 1)
	require.Equal(t, candles[31].OpenTime, signals[0].Time)
	require.Equal(t, candles[30].Low, signals[0].Stop)
	require.Greater(t, signals[0].TP2, signals[0].TP1)
	require.Equal(t, candles[31].OpenTime.Add(CapitulationTimeExit), signals[0].TimeExitAt)
}

func TestCapitulationReversalV1RejectsInvalidParametersAndNoVolume(t *testing.T) {
	_, err := CapitulationReversalV1Signals(nil, CapitulationReversalV1Params{ReturnPercent: 0, VolumeMultiplier: 2})
	require.Error(t, err)
	candles := makeCapitulationCandles(time.Now().UTC(), 50)
	candles[30].Volume = 0
	signals, err := capitulationReversalV1Signals(candles, CapitulationReversalV1Params{ReturnPercent: 4, VolumeMultiplier: 2})
	require.NoError(t, err)
	require.Empty(t, signals)
}

func TestCapitulationReversalV1NeverUsesFutureBars(t *testing.T) {
	candles := makeCapitulationCandles(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 50)
	params := CapitulationReversalV1Params{ReturnPercent: 4, VolumeMultiplier: 2}
	before, err := capitulationReversalV1Signals(candles, params)
	require.NoError(t, err)

	changed := append([]model.Candle(nil), candles...)
	for i := 40; i < len(changed); i++ {
		changed[i].Low /= 4
		changed[i].Close *= 3
	}
	after, err := capitulationReversalV1Signals(changed, params)
	require.NoError(t, err)
	require.Equal(t, signalsBefore(before, changed[40].OpenTime), signalsBefore(after, changed[40].OpenTime))
}

func makeCapitulationCandles(start time.Time, count int) []model.Candle {
	result := make([]model.Candle, count)
	price := 100.0
	for i := range result {
		price += .1
		volume := 10.0
		if i == 30 {
			price = 94
			volume = 30
		}
		if i == 31 {
			price = 96
		}
		result[i] = model.Candle{OpenTime: start.Add(time.Duration(i) * time.Hour), Open: price - .3, High: price + .5, Low: price - .6, Close: price, Volume: volume}
	}
	return result
}

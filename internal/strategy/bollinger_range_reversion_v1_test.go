package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestBollingerRangeReversionV1SignalsReentryInRange(t *testing.T) {
	candles := makeBollingerRangeCandles(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 100)
	signals, err := bollingerRangeReversionV1Signals(candles, BollingerRangeReversionV1Params{ADXMaximum: 25, BandStdDeviation: 2})
	require.NoError(t, err)
	require.Len(t, signals, 1)
	require.Equal(t, candles[91].OpenTime, signals[0].Time)
	require.Less(t, signals[0].Stop, signals[0].EntryPrice)
	require.Greater(t, signals[0].TP2, signals[0].TP1)
	require.Equal(t, candles[91].OpenTime.Add(BollingerRangeTimeExit), signals[0].TimeExitAt)
}

func TestBollingerRangeReversionV1RejectsInvalidParametersAndShortHistory(t *testing.T) {
	_, err := BollingerRangeReversionV1Signals(nil, BollingerRangeReversionV1Params{ADXMaximum: 0, BandStdDeviation: 2})
	require.Error(t, err)
	signals, err := bollingerRangeReversionV1Signals(makeBollingerRangeCandles(time.Now().UTC(), 27), BollingerRangeReversionV1Params{ADXMaximum: 25, BandStdDeviation: 2})
	require.NoError(t, err)
	require.Empty(t, signals)
}

func TestBollingerRangeReversionV1NeverUsesFutureBars(t *testing.T) {
	candles := makeBollingerRangeCandles(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 100)
	params := BollingerRangeReversionV1Params{ADXMaximum: 25, BandStdDeviation: 2}
	before, err := bollingerRangeReversionV1Signals(candles, params)
	require.NoError(t, err)

	changed := append([]model.Candle(nil), candles...)
	for i := 95; i < len(changed); i++ {
		changed[i].Close *= 4
		changed[i].High = changed[i].Close + 1
	}
	after, err := bollingerRangeReversionV1Signals(changed, params)
	require.NoError(t, err)
	require.Equal(t, signalsBefore(before, changed[95].OpenTime), signalsBefore(after, changed[95].OpenTime))
}

func makeBollingerRangeCandles(start time.Time, count int) []model.Candle {
	result := make([]model.Candle, count)
	for i := range result {
		price := 100.0
		if i%2 == 0 {
			price += .4
		} else {
			price -= .4
		}
		if i == 90 {
			price = 96
		}
		if i == 91 {
			price = 99
		}
		result[i] = model.Candle{OpenTime: start.Add(time.Duration(i) * time.Hour), Open: price + .1, High: price + .5, Low: price - .5, Close: price, Volume: 10}
	}
	return result
}

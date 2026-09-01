package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestRSIMeanReversionV1UsesCompletedFourHourTrend(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	fourHours := []model.Candle{
		{OpenTime: start, Close: 100},
		{OpenTime: start.Add(4 * time.Hour), Close: 101},
		{OpenTime: start.Add(8 * time.Hour), Close: 102},
	}
	require.Equal(t, 0, completedTrendIndex(fourHours, start.Add(4*time.Hour)))
	require.Equal(t, 1, completedTrendIndex(fourHours, start.Add(8*time.Hour)))
	require.Equal(t, 1, completedTrendIndex(fourHours, start.Add(11*time.Hour)))
}

func TestRSIMeanReversionV1RequiresValidParametersAndHistory(t *testing.T) {
	_, err := RSIMeanReversionV1Signals(nil, RSIMeanReversionV1Params{OversoldRSI: 35, StopATR: 1.2})
	require.Error(t, err)
	_, err = RSIMeanReversionV1Signals(makeResearchV3Candles(800*60), RSIMeanReversionV1Params{OversoldRSI: 25, StopATR: 1.2})
	require.NoError(t, err)
}

func TestRSIMeanReversionV1NeverUsesFutureBars(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	hours := makeRSITestHours(start, 1_000)
	fourHours := AggregateMinutes(hours, 240)
	params := RSIMeanReversionV1Params{OversoldRSI: 30, StopATR: 1.2}
	before, err := rsiMeanReversionV1Signals(hours, fourHours, params)
	require.NoError(t, err)
	require.NotEmpty(t, before)

	changed := append([]model.Candle(nil), hours...)
	for i := len(changed) - 5; i < len(changed); i++ {
		changed[i].Close *= 4
		changed[i].High = changed[i].Close
	}
	after, err := rsiMeanReversionV1Signals(changed, AggregateMinutes(changed, 240), params)
	require.NoError(t, err)
	cutoff := changed[len(changed)-5].OpenTime
	require.Equal(t, signalsBefore(before, cutoff), signalsBefore(after, cutoff))
}

func makeRSITestHours(start time.Time, count int) []model.Candle {
	result := make([]model.Candle, count)
	price := 100.0
	for i := range result {
		if i < count-40 {
			price += .1
		} else if i < count-16 {
			price -= 1.2
		} else {
			price += 1.4
		}
		result[i] = model.Candle{OpenTime: start.Add(time.Duration(i) * time.Hour), Open: price - .1, High: price + .3, Low: price - .4, Close: price, Volume: 10}
	}
	return result
}

func signalsBefore(signals []EntrySignal, cutoff time.Time) []EntrySignal {
	result := make([]EntrySignal, 0, len(signals))
	for _, signal := range signals {
		if signal.Time.Before(cutoff) {
			result = append(result, signal)
		}
	}
	return result
}

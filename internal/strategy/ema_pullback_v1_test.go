package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestEMAPullbackV1SignalsRequiresLaterRecovery(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	hours, fourHours := makeEMAPullbackCandles(start)
	params := EMAPullbackV1Params{PullbackEMAPeriod: 20, StopATR: 1.5}

	signals, err := emaPullbackV1Signals(hours, fourHours, params)
	require.NoError(t, err)
	require.Len(t, signals, 1)
	require.Equal(t, hours[1041].OpenTime, signals[0].Time)
	require.Greater(t, signals[0].EntryPrice, signals[0].Diagnostics["pullback_ema"])
	require.Greater(t, signals[0].TP2, signals[0].TP1)
	require.Equal(t, hours[1041].OpenTime.Add(EMAPullbackTimeExit), signals[0].TimeExitAt)
}

func TestEMAPullbackV1SignalsRejectsSameBarRecovery(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	hours, fourHours := makeEMAPullbackCandles(start)
	hours[1041] = model.Candle{OpenTime: hours[1041].OpenTime, Open: 149.8, High: 150.1, Low: 149.7, Close: 149.9, Volume: 10}

	signals, err := emaPullbackV1Signals(hours, fourHours, EMAPullbackV1Params{PullbackEMAPeriod: 20, StopATR: 1.5})
	require.NoError(t, err)
	require.Empty(t, signals)
}

func TestEMAPullbackV1SignalsNeverUsesFutureBars(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	hours, fourHours := makeEMAPullbackCandles(start)
	params := EMAPullbackV1Params{PullbackEMAPeriod: 20, StopATR: 1.5}
	before, err := emaPullbackV1Signals(hours, fourHours, params)
	require.NoError(t, err)

	changed := append([]model.Candle(nil), hours...)
	for i := 1050; i < len(changed); i++ {
		changed[i].High *= 10
		changed[i].Close *= 10
	}
	after, err := emaPullbackV1Signals(changed, fourHours, params)
	require.NoError(t, err)
	require.Equal(t, signalsBefore(before, changed[1050].OpenTime), signalsBefore(after, changed[1050].OpenTime))
}

func TestEMAPullbackV1RejectsInvalidParametersAndShortHistory(t *testing.T) {
	_, err := EMAPullbackV1Signals(nil, EMAPullbackV1Params{PullbackEMAPeriod: 10, StopATR: 1.5})
	require.Error(t, err)
	signals, err := emaPullbackV1Signals(make([]model.Candle, 60), make([]model.Candle, 200), EMAPullbackV1Params{PullbackEMAPeriod: 20, StopATR: 1.5})
	require.NoError(t, err)
	require.Empty(t, signals)
}

func makeEMAPullbackCandles(start time.Time) ([]model.Candle, []model.Candle) {
	fourHours := make([]model.Candle, 270)
	for i := range fourHours {
		close := 100 + float64(i)*.2
		fourHours[i] = model.Candle{OpenTime: start.Add(time.Duration(i) * 4 * time.Hour), Open: close - .1, High: close + .4, Low: close - .4, Close: close, Volume: 10}
	}
	hours := make([]model.Candle, len(fourHours)*4)
	for i := range hours {
		hours[i] = model.Candle{OpenTime: start.Add(time.Duration(i) * time.Hour), Open: 150, High: 150.4, Low: 149.6, Close: 150, Volume: 10}
	}
	// A completed touch followed by a separate green recovery above EMA20.
	hours[1040] = model.Candle{OpenTime: hours[1040].OpenTime, Open: 150, High: 150.1, Low: 149.5, Close: 149.8, Volume: 10}
	hours[1041] = model.Candle{OpenTime: hours[1041].OpenTime, Open: 149.8, High: 150.7, Low: 149.7, Close: 150.5, Volume: 10}
	return hours, fourHours
}

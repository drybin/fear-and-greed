package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestDailyLowZoneSignalsUsesFirstEarlierLowerDailyLow(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, High: 9, Low: 5, Close: 8},
		{OpenTime: base.AddDate(0, 0, 1), High: 12, Low: 7, Close: 10},
		{OpenTime: base.AddDate(0, 0, 2), Open: 7, High: 8, Low: 6, Close: 6.5},
		{OpenTime: base.AddDate(0, 0, 2).Add(15 * time.Minute), Open: 6.5, High: 8, Low: 6.5, Close: 7.5},
	}
	signals := DailyLowZoneSignals(candles)
	require.Len(t, signals, 1)
	require.Equal(t, 5.0, signals[0].Stop)
	require.Equal(t, 12.0, signals[0].TP1)
	require.Equal(t, base.AddDate(0, 0, 2).Add(15*time.Minute), signals[0].Time)
	require.True(t, signals[0].ExitAllAtTP1)
	require.Equal(t, base.AddDate(0, 0, 4), signals[0].TimeExitAt)
}

func TestDailyLowZoneSignalsRejectsZoneBreak(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, High: 9, Low: 5, Close: 8},
		{OpenTime: base.AddDate(0, 0, 1), High: 12, Low: 7, Close: 10},
		{OpenTime: base.AddDate(0, 0, 2), Open: 7, High: 8, Low: 4.9, Close: 6},
		{OpenTime: base.AddDate(0, 0, 2).Add(15 * time.Minute), Open: 6, High: 8, Low: 6, Close: 7.5},
	}
	require.Empty(t, DailyLowZoneSignals(candles))
}

func TestDailyLowZoneThirdGreenSignalsWaitsForThirdGreenCandle(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, High: 9, Low: 5, Close: 8},
		{OpenTime: base.AddDate(0, 0, 1), High: 12, Low: 7, Close: 10},
		// The touch is not itself a confirmation candle.
		{OpenTime: base.AddDate(0, 0, 2), Open: 7, High: 8, Low: 6, Close: 6.5},
		{OpenTime: base.AddDate(0, 0, 2).Add(15 * time.Minute), Open: 6.5, High: 6.8, Low: 6.5, Close: 6.7},
		{OpenTime: base.AddDate(0, 0, 2).Add(30 * time.Minute), Open: 6.7, High: 7, Low: 6.7, Close: 6.9},
		{OpenTime: base.AddDate(0, 0, 2).Add(45 * time.Minute), Open: 6.9, High: 7.6, Low: 6.9, Close: 7.5},
	}
	signals := DailyLowZoneThirdGreenSignals(candles)
	require.Len(t, signals, 1)
	require.Equal(t, base.AddDate(0, 0, 2).Add(45*time.Minute), signals[0].Time)
}

func TestDailyLowZoneThirdGreenSignalsRejectsFirstAndSecondGreenCandles(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, High: 9, Low: 5, Close: 8},
		{OpenTime: base.AddDate(0, 0, 1), High: 12, Low: 7, Close: 10},
		{OpenTime: base.AddDate(0, 0, 2), Open: 7, High: 8, Low: 6, Close: 6.5},
		{OpenTime: base.AddDate(0, 0, 2).Add(15 * time.Minute), Open: 6.5, High: 6.8, Low: 6.5, Close: 6.7},
		{OpenTime: base.AddDate(0, 0, 2).Add(30 * time.Minute), Open: 6.7, High: 7.6, Low: 6.7, Close: 7.5},
	}
	require.Empty(t, DailyLowZoneThirdGreenSignals(candles))
}

func TestDailyLowZoneThirdGreenOnePercentSignalsUsesDynamicTarget(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, High: 9, Low: 5, Close: 8},
		{OpenTime: base.AddDate(0, 0, 1), High: 12, Low: 7, Close: 10},
		{OpenTime: base.AddDate(0, 0, 2), Open: 7, High: 8, Low: 6, Close: 6.5},
		{OpenTime: base.AddDate(0, 0, 2).Add(15 * time.Minute), Open: 6.5, High: 6.8, Low: 6.5, Close: 6.7},
		{OpenTime: base.AddDate(0, 0, 2).Add(30 * time.Minute), Open: 6.7, High: 7, Low: 6.7, Close: 6.9},
		{OpenTime: base.AddDate(0, 0, 2).Add(45 * time.Minute), Open: 6.9, High: 7.6, Low: 6.9, Close: 7.5},
	}
	signals := DailyLowZoneThirdGreenOnePercentSignals(candles)
	require.Len(t, signals, 1)
	require.Equal(t, 1.0, signals[0].TargetPercent)
	require.Zero(t, signals[0].TP1)
	require.Zero(t, signals[0].TP2)
}

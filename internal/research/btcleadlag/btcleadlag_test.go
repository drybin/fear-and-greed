package btcleadlag

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeUsesNextOpenAfterCompletedBTCImpulse(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	btc := []model.Candle{
		{OpenTime: start, Open: 100, Close: 102},
		{OpenTime: start.Add(time.Hour), Open: 102, Close: 101},
		{OpenTime: start.Add(2 * time.Hour), Open: 101, Close: 100},
	}
	alt := []model.Candle{
		{OpenTime: start, Open: 10, Close: 10},
		{OpenTime: start.Add(time.Hour), Open: 10, Close: 10.5},
		{OpenTime: start.Add(2 * time.Hour), Open: 10.5, Close: 11},
	}
	report, err := Analyze(btc, map[string][]model.Candle{"ALTUSDT": alt}, Config{ImpulseThreshold: .01, Horizons: []int{1, 2}})
	require.NoError(t, err)
	require.Len(t, report.Symbols, 1)
	require.Equal(t, 1, report.BTCUpImpulseEvents)
	require.Equal(t, 0, report.BTCDownImpulseEvents)
	require.Equal(t, 1, report.Symbols[0].Up.Events)
	require.InDelta(t, 0.05, report.Symbols[0].Up.Horizons[0].MeanReturn, 1e-9)
	require.InDelta(t, 0.1, report.Symbols[0].Up.Horizons[1].MeanReturn, 1e-9)
}

func TestAnalyzeMeasuresDownImpulseDirectionally(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	btc := []model.Candle{{OpenTime: start, Open: 100, Close: 98}}
	alt := []model.Candle{{OpenTime: start.Add(time.Hour), Open: 10, Close: 9.5}}
	report, err := Analyze(btc, map[string][]model.Candle{"ALTUSDT": alt}, Config{ImpulseThreshold: .01, Horizons: []int{1}})
	require.NoError(t, err)
	require.Equal(t, 1, report.Symbols[0].Down.Events)
	require.InDelta(t, .05, report.Symbols[0].Down.Horizons[0].MeanReturn, 1e-9)
}

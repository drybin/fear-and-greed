package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestIsATRCompressionBar(t *testing.T) {
	atr := make([]float64, 10)
	for i := range atr {
		atr[i] = 10 + float64(i)
	}
	atr[9] = 5
	if !isATRCompressionBar(atr, 9, 10, 0.6) {
		t.Fatal("expected compression at min ATR bar")
	}
	atr[9] = 6
	atr[8] = 4
	if isATRCompressionBar(atr, 9, 10, 0.6) {
		t.Fatal("index 9 should not be min ATR when 8 is lower")
	}
}

func TestCompressionRangeHL(t *testing.T) {
	candles := []model.Candle{
		{High: 110, Low: 100},
		{High: 115, Low: 105},
		{High: 108, Low: 102},
	}
	h, l, ok := compressionRangeHL(candles, 2, 3)
	if !ok || h != 115 || l != 100 {
		t.Fatalf("got high=%f low=%f ok=%v", h, l, ok)
	}
}

func TestSimulateVolatilityCompressionBreakoutV1_runs(t *testing.T) {
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var hourly []model.Candle
	p := 300.0
	for i := 0; i < 400; i++ {
		hourly = append(hourly, model.Candle{
			OpenTime: base.Add(time.Duration(i) * time.Hour),
			Open: p, High: p + 5, Low: p - 3, Close: p + 2,
		})
		p += 0.2
	}
	var candles []model.Candle
	for _, c := range hourly {
		for m := 0; m < 60; m++ {
			candles = append(candles, model.Candle{
				OpenTime: c.OpenTime.Add(time.Duration(m) * time.Minute),
				Open: c.Open, High: c.High, Low: c.Low, Close: c.Close,
			})
		}
	}
	rep := SimulateVolatilityCompressionBreakoutV1(candles)
	if rep.FinalCash <= 0 {
		t.Fatal("expected valid report")
	}
}

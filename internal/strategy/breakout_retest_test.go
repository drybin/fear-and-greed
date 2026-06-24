package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func append15mBar(start time.Time, o, h, l, c float64) []model.Candle {
	var out []model.Candle
	for m := 0; m < 15; m++ {
		out = append(out, model.Candle{
			OpenTime: start.Add(time.Duration(m) * time.Minute),
			Open:     o,
			High:     h,
			Low:      l,
			Close:    c,
		})
	}
	return out
}

func TestSimulateBreakoutRetestLong_runs(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	t0 := start
	for i := 0; i < 50; i++ {
		candles = append(candles, append15mBar(t0, 100, 100.5, 99.5, 100)...)
		t0 = t0.Add(15 * time.Minute)
	}
	// swing high island: low high on neighbors
	candles = append(candles, append15mBar(t0, 100, 101, 99.8, 100)...)
	t0 = t0.Add(15 * time.Minute)
	candles = append(candles, append15mBar(t0, 100, 101, 99.8, 100)...)
	t0 = t0.Add(15 * time.Minute)
	candles = append(candles, append15mBar(t0, 100, 108, 99.8, 107)...) // pivot high 108
	t0 = t0.Add(15 * time.Minute)
	for i := 0; i < 3; i++ {
		candles = append(candles, append15mBar(t0, 107, 107.5, 106.5, 107)...)
		t0 = t0.Add(15 * time.Minute)
	}
	// breakout
	candles = append(candles, append15mBar(t0, 107, 112, 106.8, 111)...)
	t0 = t0.Add(15 * time.Minute)
	// retest zone near 108
	candles = append(candles, append15mBar(t0, 110, 110.5, 107.5, 109)...)
	t0 = t0.Add(15 * time.Minute)
	// confirm
	candles = append(candles, append15mBar(t0, 109, 111, 108.5, 110.5)...)
	t0 = t0.Add(15 * time.Minute)
	// rally to TP
	for i := 0; i < 5; i++ {
		candles = append(candles, append15mBar(t0, 110, 115, 109.5, 114)...)
		t0 = t0.Add(15 * time.Minute)
	}

	rep := SimulateBreakoutRetestLong(candles)
	if rep.CompletedCount < 1 && !rep.OpenPosition {
		t.Fatalf("expected trade activity, completed=%d open=%v", rep.CompletedCount, rep.OpenPosition)
	}
}

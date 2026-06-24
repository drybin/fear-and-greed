package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestEMA_seedsAfterPeriod(t *testing.T) {
	candles := make([]model.Candle, 210)
	for i := range candles {
		candles[i] = model.Candle{OpenTime: time.Date(2025, 1, 1, 0, i, 0, 0, time.UTC), Close: 100 + float64(i)*0.1}
	}
	ema := EMA(candles, 200)
	if ema[199] <= 0 {
		t.Fatalf("expected EMA at 199, got %f", ema[199])
	}
	if ema[209] <= ema[199] {
		t.Fatalf("expected rising EMA")
	}
}

func TestSimulateBreakoutRetestLongV2_runs(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	t0 := start
	// ~220 15m bars of uptrend for EMA200 + volume
	for i := 0; i < 250; i++ {
		p := 100 + float64(i)*0.05
		candles = append(candles, append15mBarVol(t0, p, p+0.3, p-0.2, p+0.1, 1000)...)
		t0 = t0.Add(15 * time.Minute)
	}
	rep := SimulateBreakoutRetestLongV2(candles)
	_ = rep // smoke: no panic
}

func append15mBarVol(start time.Time, o, h, l, c, vol float64) []model.Candle {
	var out []model.Candle
	for m := 0; m < 15; m++ {
		out = append(out, model.Candle{
			OpenTime: start.Add(time.Duration(m) * time.Minute),
			Open:     o,
			High:     h,
			Low:      l,
			Close:    c,
			Volume:   vol,
		})
	}
	return out
}

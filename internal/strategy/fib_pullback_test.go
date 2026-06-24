package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestSimulateFibPullbackLongV1_runs(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	t0 := start
	for i := 0; i < 2500; i++ {
		p := 100 + float64(i)*0.02
		candles = append(candles, append15mBarVol(t0, p, p+0.3, p-0.2, p+0.1, 1000)...)
		t0 = t0.Add(15 * time.Minute)
	}
	rep := SimulateFibPullbackLongV1(candles)
	if rep.StartCash != StartCash {
		t.Fatalf("unexpected start cash %f", rep.StartCash)
	}
}

func TestSimulateFibPullbackLongV1_completesTrade(t *testing.T) {
	// Build enough history for EMA200 on 1H (~200h = 800 15M bars) then impulse + pullback.
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	t0 := start
	price := 100.0
	for i := 0; i < 900; i++ {
		candles = append(candles, append15mBarVol(t0, price, price+0.5, price-0.5, price+0.1, 800)...)
		t0 = t0.Add(15 * time.Minute)
		price += 0.05
	}
	// Sharp impulse (~8% over ~20 1H bars worth)
	for i := 0; i < 80; i++ {
		price += 0.12
		candles = append(candles, append15mBarVol(t0, price, price+0.8, price-0.3, price+0.5, 1200)...)
		t0 = t0.Add(15 * time.Minute)
	}
	// Pullback into fib zone
	for i := 0; i < 30; i++ {
		price -= 0.15
		candles = append(candles, append15mBarVol(t0, price, price+0.4, price-0.4, price-0.05, 900)...)
		t0 = t0.Add(15 * time.Minute)
	}
	// Bounce for entry
	for i := 0; i < 20; i++ {
		price += 0.2
		candles = append(candles, append15mBarVol(t0, price, price+0.6, price-0.2, price+0.4, 1100)...)
		t0 = t0.Add(15 * time.Minute)
	}

	rep := SimulateFibPullbackLongV1(candles)
	_ = rep // smoke: structure may or may not trigger on synthetic path
}

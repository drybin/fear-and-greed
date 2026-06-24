package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestBullishFVG_touchZone(t *testing.T) {
	z := FibPullbackTrendZone{TopRatio: 0.5, BottomRatio: 0.618}
	legHigh := 110.0
	legLow := 100.0
	rng := legHigh - legLow
	top := legHigh - z.TopRatio*rng
	bottom := legHigh - z.BottomRatio*rng

	touch := func(low, high float64) bool {
		return low <= top && high >= bottom
	}
	if !touch(105, 106) {
		t.Fatal("expected touch in fib zone")
	}
	if touch(104, 104.5) {
		t.Fatal("expected no touch when high below zone bottom")
	}
}

func TestSimulateFibPullbackTrendV1_runs(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	t0 := start
	for i := 0; i < 2500; i++ {
		p := 100 + float64(i)*0.02
		candles = append(candles, append15mBarVol(t0, p, p+0.3, p-0.2, p+0.1, 1000)...)
		t0 = t0.Add(15 * time.Minute)
	}
	rep := SimulateFibPullbackTrendV1(candles)
	if rep.StartCash != StartCash {
		t.Fatalf("unexpected start cash %f", rep.StartCash)
	}
}

func TestSimulateFibPullbackTrendV1WithParams_sweepGrid(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	t0 := start
	for i := 0; i < 1200; i++ {
		p := 100 + float64(i)*0.03
		candles = append(candles, append15mBarVol(t0, p, p+0.4, p-0.2, p+0.15, 1000)...)
		t0 = t0.Add(15 * time.Minute)
	}
	for _, imp := range []float64{5, 8, 10, 15} {
		rep := SimulateFibPullbackTrendV1WithParams(candles, FibPullbackTrendV1Params{
			PivotLength:    5,
			MinImpulsePct:  imp,
			Zone:           FibPullbackTrendZone{TopRatio: 0.5, BottomRatio: 0.618},
			MaxWaitBars15M: 48,
		})
		if rep.FinalCash <= 0 {
			t.Fatalf("impulse %v: invalid report", imp)
		}
	}
}

func TestFibPullbackTrendZoneLabel(t *testing.T) {
	z := FibPullbackTrendZone{TopRatio: 0.5, BottomRatio: 0.618}
	if z.Label() != "0.500-0.618" {
		t.Fatalf("got %q", z.Label())
	}
}

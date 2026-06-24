package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestSimulateFibPullbackLongV2_runs(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	t0 := start
	for i := 0; i < 2500; i++ {
		p := 100 + float64(i)*0.02
		candles = append(candles, append15mBarVol(t0, p, p+0.3, p-0.2, p+0.1, 1000)...)
		t0 = t0.Add(15 * time.Minute)
	}
	rep := SimulateFibPullbackLongV2(candles)
	if rep.StartCash != StartCash {
		t.Fatalf("unexpected start cash %f", rep.StartCash)
	}
}

func TestSimulateFibPullbackLongV2_impulseSweep(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	t0 := start
	for i := 0; i < 2500; i++ {
		p := 100 + float64(i)*0.02
		candles = append(candles, append15mBarVol(t0, p, p+0.3, p-0.2, p+0.1, 1000)...)
		t0 = t0.Add(15 * time.Minute)
	}
	for _, imp := range []float64{6, 8, 10} {
		_ = SimulateFibPullbackLongV2WithParams(candles, FibPullbackV2Params{MinImpulsePct: imp})
	}
}

package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestLowestLowBefore(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Low: 110, High: 112, Close: 111},
		{OpenTime: base.Add(time.Hour), Low: 105, High: 111, Close: 110},
		{OpenTime: base.Add(2 * time.Hour), Low: 100, High: 108, Close: 107},
	}
	got, ok := lowestLowBefore(candles, 2, 2)
	if !ok || got != 105 {
		t.Fatalf("got %f ok=%v", got, ok)
	}
}

func minuteSeriesFromHourly(hourly []model.Candle) []model.Candle {
	var out []model.Candle
	for _, c := range hourly {
		for m := 0; m < 60; m++ {
			out = append(out, model.Candle{
				OpenTime: c.OpenTime.Add(time.Duration(m) * time.Minute),
				Open:     c.Open, High: c.High, Low: c.Low, Close: c.Close,
			})
		}
	}
	return out
}

func TestSimulateLiquiditySweepLongV1_runs(t *testing.T) {
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var hourly []model.Candle
	p := 300.0
	for i := 0; i < 300; i++ {
		hourly = append(hourly, model.Candle{
			OpenTime: base.Add(time.Duration(i) * time.Hour),
			Open: p, High: p + 5, Low: p - 3, Close: p + 2,
		})
		p += 0.5
	}
	rep := SimulateLiquiditySweepLongV1(minuteSeriesFromHourly(hourly))
	if rep.FinalCash <= 0 {
		t.Fatal("expected valid report")
	}
}

func TestSimulateLiquiditySweepLongV2_runs(t *testing.T) {
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var hourly []model.Candle
	p := 300.0
	for i := 0; i < 350; i++ {
		hourly = append(hourly, model.Candle{
			OpenTime: base.Add(time.Duration(i) * time.Hour),
			Open: p, High: p + 5, Low: p - 3, Close: p + 2,
		})
		p += 0.3
	}
	rep := SimulateLiquiditySweepLongV2(minuteSeriesFromHourly(hourly))
	if rep.FinalCash <= 0 {
		t.Fatal("expected valid report")
	}
}

func TestSimulateLiquiditySweepLongV3_runs(t *testing.T) {
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var hourly []model.Candle
	p := 300.0
	for i := 0; i < 350; i++ {
		hourly = append(hourly, model.Candle{
			OpenTime: base.Add(time.Duration(i) * time.Hour),
			Open: p, High: p + 5, Low: p - 3, Close: p + 2,
		})
		p += 0.3
	}
	rep := SimulateLiquiditySweepLongV3(minuteSeriesFromHourly(hourly))
	if rep.FinalCash <= 0 {
		t.Fatal("expected valid report")
	}
}

func TestSimulateLiquiditySweepLongV4_runs(t *testing.T) {
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var hourly []model.Candle
	p := 300.0
	for i := 0; i < 350; i++ {
		hourly = append(hourly, model.Candle{
			OpenTime: base.Add(time.Duration(i) * time.Hour),
			Open: p, High: p + 5, Low: p - 3, Close: p + 2,
		})
		p += 0.3
	}
	rep := SimulateLiquiditySweepLongV4(minuteSeriesFromHourly(hourly))
	if rep.FinalCash <= 0 {
		t.Fatal("expected valid report")
	}
}

func TestBullishFVG(t *testing.T) {
	candles := []model.Candle{
		{High: 100, Low: 98},
		{Open: 100, High: 110, Low: 99, Close: 108},
		{High: 112, Low: 105, Close: 111},
	}
	bottom, top, ok := bullishFVG(candles, 2)
	if !ok || bottom != 100 || top != 105 {
		t.Fatalf("got bottom=%f top=%f ok=%v", bottom, top, ok)
	}
}

func TestFvgRetestEntry(t *testing.T) {
	c := model.Candle{Open: 104, High: 108, Low: 105, Close: 107}
	if !fvgRetestEntry(c, 100, 106) {
		t.Fatal("expected retest entry")
	}
	c2 := model.Candle{Open: 107, High: 108, Low: 106, Close: 106.5}
	if fvgRetestEntry(c2, 100, 106) {
		t.Fatal("bearish close should reject")
	}
}

func TestSimulateLiquiditySweepLongV5_runs(t *testing.T) {
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var hourly []model.Candle
	p := 300.0
	for i := 0; i < 350; i++ {
		hourly = append(hourly, model.Candle{
			OpenTime: base.Add(time.Duration(i) * time.Hour),
			Open: p, High: p + 5, Low: p - 3, Close: p + 2,
		})
		p += 0.3
	}
	rep := SimulateLiquiditySweepLongV5(minuteSeriesFromHourly(hourly))
	if rep.FinalCash <= 0 {
		t.Fatal("expected valid report")
	}
}

func TestIsDisplacementBar(t *testing.T) {
	c := model.Candle{Open: 100, High: 110, Low: 99, Close: 108}
	if !isDisplacementBar(c, 5, 1.5) {
		t.Fatal("expected displacement")
	}
	c2 := model.Candle{Open: 100, High: 102, Low: 99, Close: 101}
	if isDisplacementBar(c2, 5, 1.5) {
		t.Fatal("range too small")
	}
}

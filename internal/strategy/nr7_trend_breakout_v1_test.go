package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestIsNRCandle(t *testing.T) {
	candles := []model.Candle{
		{High: 110, Low: 100},
		{High: 108, Low: 101},
		{High: 107, Low: 102},
		{High: 106, Low: 103},
		{High: 105, Low: 104},
		{High: 104, Low: 103.5},
		{High: 103.2, Low: 103.0}, // NR7: smallest range
	}
	if !isNRCandle(candles, 6, 7) {
		t.Fatal("expected NR7 at index 6")
	}
	if isNRCandle(candles, 5, 7) {
		t.Fatal("index 5 should not be NR7")
	}
}

func TestNR7TrendFilterLabel(t *testing.T) {
	if NR7TrendBoth.Label() != "close>EMA200+EMA50>EMA200" {
		t.Fatalf("got %q", NR7TrendBoth.Label())
	}
}

func TestSimulateNR7TrendBreakoutV1_runs(t *testing.T) {
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
	var candles []model.Candle
	for _, c := range hourly {
		for m := 0; m < 60; m++ {
			candles = append(candles, model.Candle{
				OpenTime: c.OpenTime.Add(time.Duration(m) * time.Minute),
				Open: c.Open, High: c.High, Low: c.Low, Close: c.Close,
			})
		}
	}
	rep := SimulateNR7TrendBreakoutV1(candles)
	if rep.FinalCash <= 0 {
		t.Fatal("expected valid report")
	}
}

func TestSimulateNR7TrendBreakoutV1_sweepParams(t *testing.T) {
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var hourly []model.Candle
	p := 300.0
	for i := 0; i < 300; i++ {
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
	for _, n := range []int{5, 7, 10} {
		rep := SimulateNR7TrendBreakoutV1WithParams(candles, NR7TrendBreakoutV1Params{
			NRLength: n, ATRCompression: 0.8, SetupLifetime: 12, TrendFilter: NR7TrendCloseEMA200,
		})
		if rep.StartCash != StartCash {
			t.Fatalf("NR%d: bad start cash", n)
		}
	}
}

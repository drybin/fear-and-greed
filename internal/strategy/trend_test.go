package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestTrendRegime(t *testing.T) {
	day := time.Date(2025, 3, 1, 0, 0, 0, 0, time.UTC)
	prev := day.AddDate(0, 0, -1)
	closes := map[time.Time]float64{prev: 110}
	sma := map[time.Time]float64{prev: 100}
	bull, ok := trendRegime(day, closes, sma)
	if !ok || !bull {
		t.Fatalf("expected bullish, got bull=%v ok=%v", bull, ok)
	}
	closes[prev] = 90
	bull, ok = trendRegime(day, closes, sma)
	if !ok || bull {
		t.Fatalf("expected bearish, got bull=%v ok=%v", bull, ok)
	}
	closes[prev] = 100
	_, ok = trendRegime(day, closes, sma)
	if ok {
		t.Fatal("expected skip when close == sma")
	}
}

func TestSimulateTrendAdaptive_longOnBull(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	// Build 55 days of bull context: prev day close > sma
	var candles []model.Candle
	for d := 0; d < 55; d++ {
		day := base.AddDate(0, 0, d)
		price := 80.0 + float64(d)*2
		candles = append(candles, model.Candle{OpenTime: day, Close: price})
	}
	// entry day 55: minute data
	entryDay := base.AddDate(0, 0, 55)
	candles = append(candles,
		model.Candle{OpenTime: entryDay, Close: 200},
		model.Candle{OpenTime: entryDay.Add(time.Minute), Close: 210},
	)
	rep := SimulateTrendAdaptive(candles, 999, TrendParams{LongTargetPct: 5, ShortTargetPct: 5})
	if rep.CompletedCount != 1 {
		t.Fatalf("expected 1 long trade, got %d liq=%d", rep.CompletedCount, rep.LiquidationCount)
	}
	if rep.RealizedCash <= StartCash {
		t.Fatalf("expected long profit, cash=%f", rep.RealizedCash)
	}
}

func TestSimulateTrendAdaptive_shortLiquidation(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	for d := 0; d < 60; d++ {
		close := 100.0
		if d >= 59 {
			close = 80
		}
		candles = append(candles, model.Candle{OpenTime: base.AddDate(0, 0, d), Close: close})
	}
	entryDay := base.AddDate(0, 0, 60)
	candles = append(candles,
		model.Candle{OpenTime: entryDay, Close: 100},
		model.Candle{OpenTime: entryDay.Add(time.Minute), Close: 250},
	)
	rep := SimulateTrendAdaptive(candles, 999, TrendParams{LongTargetPct: 5, ShortTargetPct: 2})
	if rep.LiquidationCount != 1 {
		t.Fatalf("expected 1 liquidation, got liq=%d trades=%d", rep.LiquidationCount, rep.CompletedCount)
	}
}

func TestSimulateTrendLongOnlySMA_acceptsPeriod(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	for d := 0; d < 55; d++ {
		candles = append(candles, model.Candle{
			OpenTime: base.AddDate(0, 0, d),
			Close:    80 + float64(d)*2,
		})
	}
	entryDay := base.AddDate(0, 0, 55)
	candles = append(candles,
		model.Candle{OpenTime: entryDay, Close: 200},
		model.Candle{OpenTime: entryDay.Add(time.Minute), Close: 210},
	)
	rep := SimulateTrendLongOnlySMA(candles, 999, 5, 20)
	if rep.CompletedCount < 1 {
		t.Fatalf("expected at least 1 trade with SMA20, got %d", rep.CompletedCount)
	}
}

func TestSimulateTrendLongOnly_skipsBear(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	for d := 0; d < 60; d++ {
		candles = append(candles, model.Candle{
			OpenTime: base.AddDate(0, 0, d),
			Close:    100,
		})
	}
	// force bearish regime on prev day for entry day 60
	candles[len(candles)-1].Close = 90
	entryDay := base.AddDate(0, 0, 60)
	candles = append(candles,
		model.Candle{OpenTime: entryDay, Close: 100},
		model.Candle{OpenTime: entryDay.Add(time.Minute), Close: 200},
	)

	rep := SimulateTrendLongOnly(candles, 999, 5)
	if rep.CompletedCount != 0 {
		t.Fatalf("expected no trades in bearish day, got %d", rep.CompletedCount)
	}
}

func TestSimulateTrendLongOnlySMASweepWithCache_matchesSingleRuns(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	for d := 0; d < 70; d++ {
		day := base.AddDate(0, 0, d)
		close := 100.0 + float64(d%15)
		candles = append(candles,
			model.Candle{OpenTime: day, Close: close},
			model.Candle{OpenTime: day.Add(20 * time.Minute), Close: close + 0.8},
			model.Candle{OpenTime: day.Add(40 * time.Minute), Close: close + 1.6},
		)
	}

	targets := []float64{2, 5, 9}
	targetInts := []int{2, 5, 9}
	seeds := []int64{111, 222, 333}
	smaPeriod := 20
	cache := NewTrendDailyCache(candles, []int{smaPeriod})

	got := SimulateTrendLongOnlySMASweepWithCache(candles, smaPeriod, targets, seeds, cache)
	if len(got) != len(targets) {
		t.Fatalf("expected %d reports, got %d", len(targets), len(got))
	}

	for i := range targets {
		want := SimulateTrendLongOnlySMAWithCache(candles, seeds[i], float64(targetInts[i]), smaPeriod, cache)
		if got[i].CompletedCount != want.CompletedCount {
			t.Fatalf("target %d: completed mismatch got=%d want=%d", targetInts[i], got[i].CompletedCount, want.CompletedCount)
		}
		if got[i].OpenPosition != want.OpenPosition {
			t.Fatalf("target %d: open position mismatch got=%v want=%v", targetInts[i], got[i].OpenPosition, want.OpenPosition)
		}
		if got[i].ProfitUSD != want.ProfitUSD {
			t.Fatalf("target %d: profit usd mismatch got=%f want=%f", targetInts[i], got[i].ProfitUSD, want.ProfitUSD)
		}
		if got[i].ProfitPct != want.ProfitPct {
			t.Fatalf("target %d: profit pct mismatch got=%f want=%f", targetInts[i], got[i].ProfitPct, want.ProfitPct)
		}
	}
}

func TestSimulateTrendLongRetestSMAWithCache_breakoutRetestEntry(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	for d := 0; d < 55; d++ {
		close := 100.0
		day := base.AddDate(0, 0, d)
		candles = append(candles, model.Candle{
			OpenTime: day,
			Open:     close,
			High:     close + 0.5,
			Low:      close - 0.5,
			Close:    close,
		})
	}
	// Make previous day bullish vs SMA so day 55 is eligible.
	candles[len(candles)-1].Close = 102
	candles[len(candles)-1].High = 102.5
	candles[len(candles)-1].Low = 101.5

	day := base.AddDate(0, 0, 55)
	candles = append(candles,
		model.Candle{OpenTime: day, Open: 99.5, High: 99.8, Low: 99.2, Close: 99.5},                        // below SMA
		model.Candle{OpenTime: day.Add(time.Minute), Open: 99.5, High: 101.5, Low: 99.4, Close: 101},       // breakout by close
		model.Candle{OpenTime: day.Add(2 * time.Minute), Open: 101, High: 101.2, Low: 99.95, Close: 100.2}, // retest touch
		model.Candle{OpenTime: day.Add(3 * time.Minute), Open: 100.2, High: 108, Low: 100.1, Close: 108},   // target hit
	)

	cache := NewTrendDailyCache(candles, []int{20})
	rep := SimulateTrendLongRetestSMAWithCache(candles, 1, 5, 20, 0.2, 5, cache)
	if rep.CompletedCount < 1 {
		t.Fatalf("expected at least 1 trade, got %d", rep.CompletedCount)
	}
}

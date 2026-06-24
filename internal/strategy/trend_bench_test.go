package strategy

import (
	"fmt"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func BenchmarkTrendLongSMASweep(b *testing.B) {
	candles := benchmarkCandles(900, 12)
	smaPeriod := 50
	targets := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 15, 20, 25, 30}
	cache := NewTrendDailyCache(candles, []int{smaPeriod})

	b.Run("single-run-per-target", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j, t := range targets {
				seed := int64(1000 + i*100 + j)
				_ = SimulateTrendLongOnlySMAWithCache(candles, seed, float64(t), smaPeriod, cache)
			}
		}
	})

	b.Run("batched-sweep-per-sma", func(b *testing.B) {
		targetFloats := make([]float64, len(targets))
		for i, t := range targets {
			targetFloats[i] = float64(t)
		}

		for i := 0; i < b.N; i++ {
			seeds := make([]int64, len(targets))
			for j := range targets {
				seeds[j] = int64(1000 + i*100 + j)
			}
			_ = SimulateTrendLongOnlySMASweepWithCache(candles, smaPeriod, targetFloats, seeds, cache)
		}
	})

	b.Run("batched-sweep-per-sma-100-targets", func(b *testing.B) {
		var targetFloats []float64
		var targets100 []int
		for t := 1; t <= 100; t++ {
			targets100 = append(targets100, t)
			targetFloats = append(targetFloats, float64(t))
		}

		for i := 0; i < b.N; i++ {
			seeds := make([]int64, len(targets100))
			for j := range targets100 {
				seeds[j] = int64(2000 + i*1000 + j)
			}
			_ = SimulateTrendLongOnlySMASweepWithCache(candles, smaPeriod, targetFloats, seeds, cache)
		}
	})
}

func BenchmarkTrendLongSMASweepHeavy(b *testing.B) {
	// Approximate minute data for 4 years: 1460 days * 1440 candles/day.
	candles := benchmarkCandles(1460, 1440)
	smaPeriod := 50
	cache := NewTrendDailyCache(candles, []int{smaPeriod})

	var targets []int
	var targetFloats []float64
	for t := 1; t <= 100; t++ {
		targets = append(targets, t)
		targetFloats = append(targetFloats, float64(t))
	}

	b.Run("single-run-per-target-100", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j, t := range targets {
				seed := int64(3000 + i*1000 + j)
				_ = SimulateTrendLongOnlySMAWithCache(candles, seed, float64(t), smaPeriod, cache)
			}
		}
	})

	b.Run("batched-sweep-per-sma-100", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			seeds := make([]int64, len(targets))
			for j := range targets {
				seeds[j] = int64(3000 + i*1000 + j)
			}
			_ = SimulateTrendLongOnlySMASweepWithCache(candles, smaPeriod, targetFloats, seeds, cache)
		}
	})
}

func benchmarkCandles(days, candlesPerDay int) []model.Candle {
	start := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]model.Candle, 0, days*candlesPerDay)
	for d := 0; d < days; d++ {
		dayStart := start.AddDate(0, 0, d)
		base := 100 + float64(d%120)*0.25
		for m := 0; m < candlesPerDay; m++ {
			// deterministic pseudo-wave with mild trend.
			price := base + float64((m*7+d*3)%11)*0.4 + float64((d/30)%3)*0.5
			out = append(out, model.Candle{
				OpenTime: dayStart.Add(time.Duration(m) * time.Minute),
				Close:    price,
			})
		}
	}
	if len(out) == 0 {
		panic(fmt.Errorf("benchmark candles are empty"))
	}
	return out
}

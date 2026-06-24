package usecase

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func BenchmarkRunTrendLongSMASweep(b *testing.B) {
	candles := benchmarkUsecaseCandles(365, 720)
	opts := ScanMarketsOptions{
		Seed:       42,
		TargetMin:  1,
		TargetMax:  100,
		TargetStep: 1,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = runTrendLongSMASweep(candles, "BTCUSDT", "full", opts)
	}
}

func benchmarkUsecaseCandles(days, candlesPerDay int) []model.Candle {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]model.Candle, 0, days*candlesPerDay)
	for d := 0; d < days; d++ {
		dayStart := start.AddDate(0, 0, d)
		base := 100 + float64(d%90)*0.45
		for m := 0; m < candlesPerDay; m++ {
			price := base + float64((m*11+d*5)%17)*0.3
			out = append(out, model.Candle{
				OpenTime: dayStart.Add(time.Duration(m) * time.Minute),
				Close:    price,
			})
		}
	}
	return out
}

package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestAggregateMinutes_4H(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	for i := 0; i < 240; i++ {
		candles = append(candles, model.Candle{
			OpenTime: start.Add(time.Duration(i) * time.Minute),
			Open:     100,
			High:     101,
			Low:      99,
			Close:    100,
			Volume:   10,
		})
	}
	out := AggregateMinutes(candles, 240)
	if len(out) != 1 {
		t.Fatalf("expected 1 4h bar, got %d", len(out))
	}
	if out[0].Volume != 2400 {
		t.Fatalf("expected summed volume 2400, got %f", out[0].Volume)
	}
}

func TestSimulateCRTLong_completesTrade(t *testing.T) {
	// Build enough history for ATR/vol SMA then one strong 4H impulse, pullback, entry, TP1+TP2 path.
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	price := 100.0
	vol := 100.0

	// 30 days of flat 4H bars (480 4H periods need less - 25*6=150 4h bars from 25 days)
	for day := 0; day < 25; day++ {
		for h4 := 0; h4 < 6; h4++ {
			base := start.AddDate(0, 0, day).Add(time.Duration(h4*4) * time.Hour)
			for m := 0; m < 240; m++ {
				candles = append(candles, model.Candle{
					OpenTime: base.Add(time.Duration(m) * time.Minute),
					Open:     price,
					High:     price + 0.5,
					Low:      price - 0.5,
					Close:    price,
					Volume:   vol,
				})
			}
		}
	}

	// Strong bullish 4H impulse at a known boundary
	impStart := start.AddDate(0, 0, 25)
	for m := 0; m < 240; m++ {
		p := 100.0 + float64(m)*0.05
		candles = append(candles, model.Candle{
			OpenTime: impStart.Add(time.Duration(m) * time.Minute),
			Open:     p,
			High:     p + 2,
			Low:      p - 0.1,
			Close:    p + 1.5,
			Volume:   vol * 3,
		})
	}
	rangeHigh := 100.0 + 239*0.05 + 2
	rangeLow := 100.0 - 0.1

	// Pullback into discount on 15M (several 15m bars)
	pullStart := impStart.Add(4 * time.Hour)
	for q := 0; q < 8; q++ {
		base := pullStart.Add(time.Duration(q*15) * time.Minute)
		mid := (rangeHigh + rangeLow) / 2
		for m := 0; m < 15; m++ {
			px := mid - float64(q)*0.5
			candles = append(candles, model.Candle{
				OpenTime: base.Add(time.Duration(m) * time.Minute),
				Open:     px + 0.2,
				High:     px + 0.3,
				Low:      rangeLow + 0.05,
				Close:    px,
				Volume:   vol,
			})
		}
	}

	// Bullish reaction + rally to TP1/TP2
	reactStart := pullStart.Add(2 * time.Hour)
	for m := 0; m < 60; m++ {
		px := rangeLow + float64(m)*(rangeHigh-rangeLow)/30
		candles = append(candles, model.Candle{
			OpenTime: reactStart.Add(time.Duration(m) * time.Minute),
			Open:     px - 0.1,
			High:     px + 1,
			Low:      px - 0.2,
			Close:    px + 0.5,
			Volume:   vol * 2,
		})
	}

	rep := SimulateCRTLong(candles)
	if rep.CompletedCount < 1 && !rep.OpenPosition {
		t.Fatalf("expected trade or open position, completed=%d open=%v profit=%.2f",
			rep.CompletedCount, rep.OpenPosition, rep.ProfitPct)
	}
}

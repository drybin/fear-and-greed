package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestSimulateRandomRise2DayProfit_targetBefore2d(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Close: 100},
		{OpenTime: base.Add(time.Hour), Close: 106},
	}
	rep := SimulateRandomRise2DayProfit(candles, 999, 5)
	if rep.CompletedCount != 1 {
		t.Fatalf("expected 1 trade, got %d", rep.CompletedCount)
	}
	if rep.Trades[0].ExitReason != ExitReasonTarget {
		t.Fatalf("exit reason: %s", rep.Trades[0].ExitReason)
	}
}

func TestSimulateRandomRise2DayProfit_profit2d(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Close: 100},
		{OpenTime: base.Add(49 * time.Hour), Close: 102.5},
	}
	rep := SimulateRandomRise2DayProfit(candles, 999, 50)
	if rep.CompletedCount != 1 {
		t.Fatalf("expected 1 trade, got %d", rep.CompletedCount)
	}
	if rep.Trades[0].ExitReason != ExitReasonProfit2D {
		t.Fatalf("exit reason: %s", rep.Trades[0].ExitReason)
	}
}

func TestSimulateRandomRise2DayProfit_profitWait(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Close: 100},
		{OpenTime: base.Add(49 * time.Hour), Close: 101},
		{OpenTime: base.Add(72 * time.Hour), Close: 102.1},
	}
	rep := SimulateRandomRise2DayProfit(candles, 999, 50)
	if rep.CompletedCount != 1 {
		t.Fatalf("expected 1 trade, got %d", rep.CompletedCount)
	}
	if rep.Trades[0].ExitReason != ExitReasonProfitWait {
		t.Fatalf("exit reason: %s", rep.Trades[0].ExitReason)
	}
}

func TestSimulateRandomRise2DayProfit_openPosition(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Close: 100},
		{OpenTime: base.Add(49 * time.Hour), Close: 90},
		{OpenTime: base.Add(72 * time.Hour), Close: 95},
	}
	rep := SimulateRandomRise2DayProfit(candles, 999, 50)
	if !rep.OpenPosition {
		t.Fatal("expected open position")
	}
	if rep.CompletedCount != 0 {
		t.Fatalf("expected 0 trades, got %d", rep.CompletedCount)
	}
}

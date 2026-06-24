package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestShortLiquidationPrice(t *testing.T) {
	got := shortLiquidationPrice(100, 5)
	if got < 119.9 || got > 120.1 {
		t.Fatalf("expected 120, got %f", got)
	}
}

func TestSimulateRandomShortLeveraged_coverProfit(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Close: 100},
		{OpenTime: base.Add(time.Minute), Close: 98},
	}
	rep := SimulateRandomShortLeveraged(candles, 999, ShortLeverageParams{
		TargetPct: 2, Leverage: 5, MarginUSD: 10,
	})
	if rep.CompletedCount != 1 {
		t.Fatalf("expected 1 trade, got %d", rep.CompletedCount)
	}
	if rep.LiquidationCount != 0 {
		t.Fatalf("expected no liquidation, got %d", rep.LiquidationCount)
	}
	// PnL = 10*5*2/100 = $1
	if rep.RealizedCash < 100.9 || rep.RealizedCash > 101.1 {
		t.Fatalf("expected ~101 cash, got %f", rep.RealizedCash)
	}
}

func TestSimulateRandomShortLeveraged_liquidation(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Close: 100},
		{OpenTime: base.Add(time.Minute), Close: 120}, // +20% hits 5x liq at +20%
	}
	rep := SimulateRandomShortLeveraged(candles, 999, ShortLeverageParams{
		TargetPct: 2, Leverage: 5, MarginUSD: 10,
	})
	if rep.LiquidationCount != 1 {
		t.Fatalf("expected 1 liquidation, got %d", rep.LiquidationCount)
	}
	if rep.CompletedCount != 0 {
		t.Fatalf("expected 0 covers, got %d", rep.CompletedCount)
	}
	if rep.RealizedCash != 90 {
		t.Fatalf("expected 90 cash after losing $10 margin, got %f", rep.RealizedCash)
	}
}

func TestSimulateRandomShortLeveraged_bankrupt(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Close: 100},
		{OpenTime: base.Add(time.Minute), Close: 200},
		{OpenTime: base.Add(2 * time.Minute), Close: 200},
	}
	rep := SimulateRandomShortLeveraged(candles, 999, ShortLeverageParams{
		TargetPct: 2, Leverage: 1, MarginUSD: 100,
	})
	if !rep.Bankrupt {
		t.Fatal("expected bankrupt after losing full margin")
	}
	if rep.LiquidationCount != 1 {
		t.Fatalf("expected 1 liquidation, got %d", rep.LiquidationCount)
	}
}

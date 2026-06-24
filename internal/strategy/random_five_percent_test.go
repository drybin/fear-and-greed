package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

func TestSimulateRandomTarget_simple(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Close: 100},
		{OpenTime: base.Add(time.Minute), Close: 103},
		{OpenTime: base.Add(2 * time.Minute), Close: 106},
	}
	rep := SimulateRandomTarget(candles, 999, 5)
	if rep.CompletedCount != 1 {
		t.Fatalf("expected 1 trade, got %d", rep.CompletedCount)
	}
	if rep.FinalCash <= StartCash {
		t.Fatalf("expected profit, final=%f", rep.FinalCash)
	}
}

func TestFilterLastYears(t *testing.T) {
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	for i := 0; i < 1000; i++ {
		candles = append(candles, model.Candle{OpenTime: base.Add(time.Duration(i) * time.Hour)})
	}
	filtered := FilterLastYears(candles, 2)
	if len(filtered) == 0 {
		t.Fatal("expected some candles")
	}
}

func TestSimulateRandomTargetDrop_holdsAcrossDays(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Close: 100},
		{OpenTime: base.Add(12 * time.Hour), Close: 102},
		{OpenTime: base.Add(24 * time.Hour), Close: 98},
		{OpenTime: base.Add(25 * time.Hour), Close: 97},
	}
	rep := SimulateRandomTargetDrop(candles, 42, 2)
	if rep.CompletedCount != 1 {
		t.Fatalf("expected 1 trade after waiting for drop, got %d", rep.CompletedCount)
	}
}

func TestSimulateRandomTargetDrop_openPosition(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Close: 100},
		{OpenTime: base.Add(time.Minute), Close: 200},
	}
	rep := SimulateRandomTargetDrop(candles, 1, 50)
	if !rep.OpenPosition {
		t.Fatal("expected open position when drop never hit")
	}
	if rep.CompletedCount != 0 {
		t.Fatalf("expected 0 trades, got %d", rep.CompletedCount)
	}
}

func TestSimulateRandomTargetDrop_simple(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := []model.Candle{
		{OpenTime: base, Close: 100},
		{OpenTime: base.Add(time.Minute), Close: 98},
		{OpenTime: base.Add(2 * time.Minute), Close: 94},
	}
	rep := SimulateRandomTargetDrop(candles, 999, 6)
	if rep.CompletedCount != 1 {
		t.Fatalf("expected 1 trade, got %d", rep.CompletedCount)
	}
	if rep.RealizedCash <= StartCash {
		t.Fatalf("expected profit on short cover, realized=%f", rep.RealizedCash)
	}
}

func TestShortCashMTM(t *testing.T) {
	got := shortCashMTM(100, 100, 98)
	if got < 101.9 || got > 102.1 {
		t.Fatalf("expected ~102, got %f", got)
	}
	loss := shortCashMTM(100, 100, 110)
	if loss > 99 || loss < 89 {
		t.Fatalf("expected ~90 on adverse move, got %f", loss)
	}
}

func TestFillStats_profitFromRealizedCashOnly(t *testing.T) {
	rep := SimulationReport{
		StartCash:    StartCash,
		RealizedCash: 114.9, // ~7 trades × 2% compounded
		FinalCash:    37_230,
		OpenPosition: true,
	}
	rep.Trades = make([]Trade, 7)
	rep.fillStats()
	if rep.ProfitPct < 14 || rep.ProfitPct > 16 {
		t.Fatalf("expected ~15%% from realized, got %f", rep.ProfitPct)
	}
	if rep.OpenLegUSD < 37_000 {
		t.Fatalf("expected large open-leg MTM separate from realized, got %f", rep.OpenLegUSD)
	}
}

func TestFilterCurrentYear(t *testing.T) {
	candles := []model.Candle{
		{OpenTime: time.Date(2025, 12, 31, 23, 0, 0, 0, time.UTC), Close: 1},
		{OpenTime: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Close: 2},
		{OpenTime: time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), Close: 3},
	}
	filtered := FilterCurrentYear(candles)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 candles in 2026, got %d", len(filtered))
	}
}

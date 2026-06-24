package usecase

import (
	"testing"

	"github.com/drybin/fear-and-greed/internal/strategy"
)

func TestBestSweepRow_ignoresOpenLegFlagWhenTradesExist(t *testing.T) {
	rows := []sweepRow{
		{target: 1, report: strategy.SimulationReport{CompletedCount: 0, OpenPosition: true, ProfitPct: -26}},
		{target: 5, report: strategy.SimulationReport{CompletedCount: 10, OpenPosition: true, ProfitPct: 42}},
	}
	best := bestSweepRow(rows, 1)
	if best == nil || best.target != 5 {
		t.Fatalf("expected target 5, got %v", best)
	}
}

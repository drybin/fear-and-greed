package scanreport_test

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/infrastructure/scanreport"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

func TestTradesFromStrategy(t *testing.T) {
	rep := strategy.SimulationReport{
		Trades: []strategy.Trade{{
			BuyTime:      time.Date(2024, 3, 1, 10, 0, 0, 0, time.UTC),
			SellTime:     time.Date(2024, 3, 1, 18, 0, 0, 0, time.UTC),
			BuyPrice:     100,
			SellPrice:    105,
			ExitReason:   strategy.ExitReasonTP2,
			EntryContext: map[string]float64{"sl": 95, "tp2": 110},
			ExitContext:  map[string]float64{"close": 105, "tp2": 110},
			Events: []strategy.TradeEvent{{
				Kind: strategy.TradeEventTP1Partial, Time: time.Date(2024, 3, 1, 14, 0, 0, 0, time.UTC),
				Price: 102, Fraction: 0.5,
			}},
		}},
	}
	rows := scanreport.TradesFromStrategy(rep.Trades)
	if len(rows) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(rows))
	}
	if rows[0].ExitReason != strategy.ExitReasonTP2 {
		t.Fatalf("exit reason: %s", rows[0].ExitReason)
	}
	// blended: 0.5*1.02 + 0.5*1.05 = 3.5%
	if rows[0].PnLPct < 3.49 || rows[0].PnLPct > 3.51 {
		t.Fatalf("pnl: %f", rows[0].PnLPct)
	}
	if rows[0].EntryContext["sl"] != 95 {
		t.Fatalf("context: %+v", rows[0].EntryContext)
	}
	if len(rows[0].Events) != 1 || rows[0].Events[0].Kind != strategy.TradeEventTP1Partial {
		t.Fatalf("events: %+v", rows[0].Events)
	}
	if rows[0].ExitContext["close"] != 105 {
		t.Fatalf("exit context: %+v", rows[0].ExitContext)
	}
}

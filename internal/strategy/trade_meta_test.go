package strategy

import "testing"

func TestTradeBlendedPnLPct(t *testing.T) {
	t.Run("no events uses round trip", func(t *testing.T) {
		got := TradeBlendedPnLPct(Trade{BuyPrice: 100, SellPrice: 110})
		if got < 9.99 || got > 10.01 {
			t.Fatalf("got %f", got)
		}
	})

	t.Run("50% at tp1 and 50% at final", func(t *testing.T) {
		tr := Trade{
			BuyPrice:  100,
			SellPrice: 120,
			Events: []TradeEvent{{
				Kind: TradeEventTP1Partial, Price: 110, Fraction: 0.5,
			}},
		}
		got := TradeBlendedPnLPct(tr)
		// 0.5*1.10 + 0.5*1.20 - 1 = 15%
		if got < 14.99 || got > 15.01 {
			t.Fatalf("got %f", got)
		}
	})

	t.Run("effective picks blended", func(t *testing.T) {
		tr := Trade{
			BuyPrice:  100,
			SellPrice: 120,
			Events: []TradeEvent{{
				Kind: TradeEventTP1Partial, Price: 110, Fraction: 0.5,
			}},
		}
		if EffectiveTradePnLPct(tr) != TradeBlendedPnLPct(tr) {
			t.Fatal("expected blended")
		}
	})
}

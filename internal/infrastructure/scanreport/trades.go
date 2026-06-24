package scanreport

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/strategy"
)

// TradeEventRecord is a partial exit within one round-trip.
type TradeEventRecord struct {
	Kind     string  `json:"kind"`
	Time     string  `json:"time"`
	Price    float64 `json:"price"`
	Fraction float64 `json:"fraction"`
}

// TradeRecord is one completed round-trip for JSON/HTML export.
type TradeRecord struct {
	EntryTime    string             `json:"entry_time"`
	ExitTime     string             `json:"exit_time"`
	EntryPrice   float64            `json:"entry_price"`
	ExitPrice    float64            `json:"exit_price"`
	WaitHours    float64            `json:"wait_hours"`
	PnLPct       float64            `json:"pnl_pct"`
	ExitReason   string             `json:"exit_reason,omitempty"`
	EntryContext map[string]float64 `json:"entry_context,omitempty"`
	ExitContext  map[string]float64 `json:"exit_context,omitempty"`
	Events       []TradeEventRecord `json:"events,omitempty"`
	VolumeRatio  float64            `json:"volume_ratio,omitempty"`
}

// TradesFromStrategy converts strategy trades to report records.
func TradesFromStrategy(trades []strategy.Trade) []TradeRecord {
	if len(trades) == 0 {
		return nil
	}
	out := make([]TradeRecord, 0, len(trades))
	for _, t := range trades {
		rec := TradeRecord{
			EntryTime:    t.BuyTime.UTC().Format(time.RFC3339),
			ExitTime:     t.SellTime.UTC().Format(time.RFC3339),
			EntryPrice:   t.BuyPrice,
			ExitPrice:    t.SellPrice,
			WaitHours:    t.WaitHours,
			PnLPct:       strategy.EffectiveTradePnLPct(t),
			ExitReason:   t.ExitReason,
			EntryContext: t.EntryContext,
			ExitContext:  t.ExitContext,
			VolumeRatio:  t.VolumeRatio,
		}
		if len(t.Events) > 0 {
			rec.Events = make([]TradeEventRecord, 0, len(t.Events))
			for _, e := range t.Events {
				rec.Events = append(rec.Events, TradeEventRecord{
					Kind:     e.Kind,
					Time:     e.Time.UTC().Format(time.RFC3339),
					Price:    e.Price,
					Fraction: e.Fraction,
				})
			}
		}
		out = append(out, rec)
	}
	return out
}

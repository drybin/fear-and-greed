package strategy

import "time"

// Exit reason labels for trade journal / reports.
const (
	ExitReasonStop        = "stop"
	ExitReasonBreakeven   = "breakeven"
	ExitReasonTP2         = "tp2"
	ExitReasonTrailEMA20  = "trail_ema20"
	ExitReasonTarget      = "target"
	ExitReasonProfit2D    = "profit_2d"
	ExitReasonProfitWait  = "profit_wait"
	ExitReasonCover       = "cover"
	ExitReasonLiquidation = "liquidation"
	TradeEventTP1Partial  = "tp1_partial"
)

// TradeEvent is a partial exit (e.g. TP1) within one round-trip.
type TradeEvent struct {
	Kind     string // tp1_partial
	Time     time.Time
	Price    float64
	Fraction float64 // share of position closed at this price
}

// EntrySignal records a close-confirmed legacy entry decision independently of
// its later exit. It lets protocol adapters reuse legacy detection logic
// without deriving entries from completed trades, which omit open positions.
//
// Time is the opening timestamp of the bar whose close confirmed the setup;
// the protocol-v2 adapter maps it to next-bar execution.
type EntrySignal struct {
	Time          time.Time
	EntryPrice    float64
	Stop          float64
	TP1           float64
	TP2           float64
	TargetPercent float64
	ExitAllAtTP1  bool
	TimeExitAt    time.Time
	Diagnostics   map[string]float64
}

// TradeRoundTripPnLPct returns (exit/entry - 1) * 100 for a simple single exit.
func TradeRoundTripPnLPct(t Trade) float64 {
	if t.BuyPrice <= 0 {
		return 0
	}
	return (t.SellPrice/t.BuyPrice - 1) * 100
}

// TradeBlendedPnLPct accounts for partial exits in Events plus final SellPrice.
func TradeBlendedPnLPct(t Trade) float64 {
	if t.BuyPrice <= 0 {
		return 0
	}
	if len(t.Events) == 0 {
		return TradeRoundTripPnLPct(t)
	}
	rem := 1.0
	sum := 0.0
	for _, e := range t.Events {
		if e.Fraction <= 0 {
			continue
		}
		sum += e.Fraction * (e.Price / t.BuyPrice)
		rem -= e.Fraction
	}
	if rem > 0 {
		sum += rem * (t.SellPrice / t.BuyPrice)
	}
	return (sum - 1) * 100
}

// EffectiveTradePnLPct picks blended PnL when partial events exist.
func EffectiveTradePnLPct(t Trade) float64 {
	if len(t.Events) > 0 {
		return TradeBlendedPnLPct(t)
	}
	return TradeRoundTripPnLPct(t)
}

// ExitCtx builds exit_context with close and optional fields.
func ExitCtx(close float64, extra map[string]float64) map[string]float64 {
	out := map[string]float64{"close": close}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// CtxFlag stores bool as 0/1 in context maps.
func CtxFlag(v bool) float64 {
	if v {
		return 1
	}
	return 0
}

// CloneContext copies a float map for trade storage.
func CloneContext(src map[string]float64) map[string]float64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// CloneEvents copies partial exit events for trade storage.
func CloneEvents(src []TradeEvent) []TradeEvent {
	if len(src) == 0 {
		return nil
	}
	return append([]TradeEvent(nil), src...)
}

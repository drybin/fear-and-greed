package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	LocalHighReversalLookback = 20
	LocalHighReversalTimeExit = 48 * time.Hour
)

// LocalHighReversalV1Signals detects a completed causal 20-hour high. The
// adapter enters short on the following hour, so future price data is unused.
func LocalHighReversalV1Signals(candles []model.Candle) []EntrySignal {
	hours := AggregateMinutes(candles, 60)
	if len(hours) <= LocalHighReversalLookback {
		return nil
	}
	signals := make([]EntrySignal, 0)
	for i := LocalHighReversalLookback; i < len(hours); i++ {
		priorHigh := hours[i-LocalHighReversalLookback].High
		for j := i - LocalHighReversalLookback + 1; j < i; j++ {
			if hours[j].High > priorHigh {
				priorHigh = hours[j].High
			}
		}
		entry, stop := hours[i].Close, hours[i].High
		risk := stop - entry
		if stop <= 0 || hours[i].High <= priorHigh || risk <= 0 || risk/entry > .15 {
			continue
		}
		signals = append(signals, EntrySignal{
			Time: hours[i].OpenTime, EntryPrice: entry, Stop: stop,
			TP1: entry - risk, TP2: entry - 2*risk,
			TimeExitAt:  hours[i].OpenTime.Add(LocalHighReversalTimeExit),
			Diagnostics: map[string]float64{"local_high": stop, "prior_high_20h": priorHigh, "lookback_hours": LocalHighReversalLookback},
		})
	}
	return signals
}

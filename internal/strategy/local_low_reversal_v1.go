package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	LocalLowReversalLookback = 20
	LocalLowReversalTimeExit = 48 * time.Hour
)

// LocalLowReversalV1Signals enters at the next 1h open after a completed hour
// makes a causal 20-hour low; no future candle identifies the setup.
func LocalLowReversalV1Signals(minutes []model.Candle) []EntrySignal {
	hours := AggregateMinutes(minutes, 60)
	if len(hours) <= LocalLowReversalLookback {
		return nil
	}
	signals := make([]EntrySignal, 0)
	for i := LocalLowReversalLookback; i < len(hours); i++ {
		priorLow := hours[i-LocalLowReversalLookback].Low
		for j := i - LocalLowReversalLookback + 1; j < i; j++ {
			if hours[j].Low < priorLow {
				priorLow = hours[j].Low
			}
		}
		entry, stop := hours[i].Close, hours[i].Low
		risk := entry - stop
		if stop <= 0 || hours[i].Low >= priorLow || risk <= 0 || risk/entry > .15 {
			continue
		}
		signals = append(signals, EntrySignal{Time: hours[i].OpenTime, EntryPrice: entry, Stop: stop, TP1: entry + risk, TP2: entry + 2*risk, TimeExitAt: hours[i].OpenTime.Add(LocalLowReversalTimeExit), Diagnostics: map[string]float64{"local_low": stop, "prior_low_20h": priorLow, "lookback_hours": LocalLowReversalLookback}})
	}
	return signals
}

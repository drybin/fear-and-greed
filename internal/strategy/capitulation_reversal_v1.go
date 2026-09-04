package strategy

import (
	"fmt"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	CapitulationVolumePeriod = 20
	CapitulationRecoveryBars = 12
	CapitulationTimeExit     = 48 * time.Hour
)

// CapitulationReversalV1Params bounds the severity and participation required
// for an event. Recovery, exits, and event lifetime are fixed behavior.
type CapitulationReversalV1Params struct {
	ReturnPercent    float64
	VolumeMultiplier float64
}

func ValidateCapitulationReversalV1Params(p CapitulationReversalV1Params) error {
	if !finiteStrategy(p.ReturnPercent) || p.ReturnPercent <= 0 ||
		!finiteStrategy(p.VolumeMultiplier) || p.VolumeMultiplier <= 1 {
		return fmt.Errorf("strategy: invalid capitulation reversal v1 parameters")
	}
	return nil
}

// CapitulationReversalV1Signals finds a completed, high-participation sell-off
// and waits for a later recovery close. Every event field is known at event
// close; recovery never consults lows, volume, or prices after its own close.
func CapitulationReversalV1Signals(minutes []model.Candle, p CapitulationReversalV1Params) ([]EntrySignal, error) {
	if err := ValidateCapitulationReversalV1Params(p); err != nil {
		return nil, err
	}
	return capitulationReversalV1Signals(AggregateMinutes(minutes, 60), p)
}

func capitulationReversalV1Signals(candles []model.Candle, p CapitulationReversalV1Params) ([]EntrySignal, error) {
	if err := ValidateCapitulationReversalV1Params(p); err != nil {
		return nil, err
	}
	if len(candles) < CapitulationVolumePeriod+2 {
		return nil, nil
	}
	volumes := candleVolumes(candles)
	volumeSMA := smaFloats(volumes, CapitulationVolumePeriod)
	type event struct {
		index       int
		close       float64
		low         float64
		returnPct   float64
		volumeRatio float64
	}
	var active *event
	signals := make([]EntrySignal, 0)
	for i := CapitulationVolumePeriod; i < len(candles); i++ {
		if active != nil && i-active.index > CapitulationRecoveryBars {
			active = nil
		}
		priorVolume := volumeSMA[i-1]
		returnPercent := (candles[i].Close/candles[i-1].Close - 1) * 100
		if candles[i-1].Close > 0 && priorVolume > 0 && candles[i].Volume > 0 &&
			returnPercent <= -p.ReturnPercent && candles[i].Volume >= priorVolume*p.VolumeMultiplier {
			active = &event{index: i, close: candles[i].Close, low: candles[i].Low, returnPct: returnPercent, volumeRatio: candles[i].Volume / priorVolume}
			continue
		}
		if active == nil || candles[i].Close <= candles[i].Open || candles[i].Close <= active.close {
			continue
		}
		entry := candles[i].Close
		risk := entry - active.low
		if active.low <= 0 || risk <= 0 || risk/entry > .15 {
			active = nil
			continue
		}
		signals = append(signals, EntrySignal{
			Time: candles[i].OpenTime, EntryPrice: entry, Stop: active.low,
			TP1: entry + risk, TP2: entry + 2*risk,
			TimeExitAt: candles[i].OpenTime.Add(CapitulationTimeExit),
			Diagnostics: map[string]float64{
				"event_return_percent": active.returnPct, "required_return_percent": p.ReturnPercent,
				"event_low": active.low, "event_close": active.close, "event_volume_ratio": active.volumeRatio,
				"required_volume_multiplier": p.VolumeMultiplier, "recovery_close": candles[i].Close,
			},
		})
		active = nil
	}
	return signals, nil
}

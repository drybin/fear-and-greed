package strategy

import (
	"fmt"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	VolumeBreakoutVolumePeriod = 20
	VolumeBreakoutATRPeriod    = 14
	VolumeBreakoutStopATR      = 1.5
	VolumeBreakoutTimeExit     = 7 * 24 * time.Hour
)

// VolumeBreakoutV1Params bounds only the causal level length and relative
// volume threshold. Stop distance, targets, and timeout are fixed behavior.
type VolumeBreakoutV1Params struct {
	ChannelBars      int
	VolumeMultiplier float64
}

func ValidateVolumeBreakoutV1Params(p VolumeBreakoutV1Params) error {
	if (p.ChannelBars != 20 && p.ChannelBars != 40) ||
		!finiteStrategy(p.VolumeMultiplier) || p.VolumeMultiplier < 1 {
		return fmt.Errorf("strategy: invalid volume breakout v1 parameters")
	}
	return nil
}

// VolumeBreakoutV1Signals emits a 1h close-confirmed breakout above a level
// built only from earlier candles. Relative volume uses the preceding 20h
// SMA, so the signal contains no future price or volume information.
func VolumeBreakoutV1Signals(minutes []model.Candle, p VolumeBreakoutV1Params) ([]EntrySignal, error) {
	if err := ValidateVolumeBreakoutV1Params(p); err != nil {
		return nil, err
	}
	return volumeBreakoutV1Signals(AggregateMinutes(minutes, 60), p)
}

func volumeBreakoutV1Signals(candles []model.Candle, p VolumeBreakoutV1Params) ([]EntrySignal, error) {
	if err := ValidateVolumeBreakoutV1Params(p); err != nil {
		return nil, err
	}
	minimum := p.ChannelBars + 1
	if VolumeBreakoutVolumePeriod+1 > minimum {
		minimum = VolumeBreakoutVolumePeriod + 1
	}
	if VolumeBreakoutATRPeriod+1 > minimum {
		minimum = VolumeBreakoutATRPeriod + 1
	}
	if len(candles) < minimum {
		return nil, nil
	}
	atr := ATRWilder(candles, VolumeBreakoutATRPeriod)
	volumeSMA := smaFloats(candleVolumes(candles), VolumeBreakoutVolumePeriod)
	signals := make([]EntrySignal, 0)
	for i := minimum - 1; i < len(candles); i++ {
		priorHigh, _, ok := priorRange(candles, i, p.ChannelBars)
		if !ok || atr[i] <= 0 || volumeSMA[i-1] <= 0 || candles[i].Volume <= 0 {
			continue
		}
		if candles[i].Close <= priorHigh || candles[i-1].Close > priorHigh || candles[i].Volume < volumeSMA[i-1]*p.VolumeMultiplier {
			continue
		}
		entry := candles[i].Close
		stop := entry - VolumeBreakoutStopATR*atr[i]
		risk := entry - stop
		if stop <= 0 || risk <= 0 || risk/entry > .15 {
			continue
		}
		signals = append(signals, EntrySignal{
			Time: candles[i].OpenTime, EntryPrice: entry, Stop: stop,
			TP1: entry + risk, TP2: entry + 3*risk,
			TimeExitAt: candles[i].OpenTime.Add(VolumeBreakoutTimeExit),
			Diagnostics: map[string]float64{
				"prior_high": priorHigh, "channel_bars": float64(p.ChannelBars),
				"volume": candles[i].Volume, "volume_sma20_prior": volumeSMA[i-1],
				"volume_multiplier": p.VolumeMultiplier, "atr14": atr[i], "stop_atr": VolumeBreakoutStopATR,
			},
		})
	}
	return signals, nil
}

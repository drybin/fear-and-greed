package strategy

import (
	"fmt"
	"math"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	ResearchV3EMAPeriod    = 200
	ResearchV3EMARiseBars  = 20
	ResearchV3ATRPeriod    = 14
	ResearchV3VolumePeriod = 20
	ResearchV3RangeBars    = 10
	ResearchV3RSIPeriod    = 14
	ResearchV3RSIRecovery  = 35
)

// VolatilityCompressionBreakoutV2Params deliberately bounds the search to
// compression and volume confirmation. Stops and targets are derived from the
// same causal range and are not tunable.
type VolatilityCompressionBreakoutV2Params struct {
	CompressionFactor float64
	VolumeMultiplier  float64
}

// MeanReversionV1Params tests a distinct hypothesis: an oversold pullback can
// recover within an established uptrend. The recovery threshold is fixed to
// keep the grid small; only oversold severity and stop distance vary.
type MeanReversionV1Params struct {
	OversoldRSI float64
	StopATR     float64
}

func ValidateVolatilityCompressionBreakoutV2Params(p VolatilityCompressionBreakoutV2Params) error {
	if !finiteStrategy(p.CompressionFactor) || p.CompressionFactor <= 0 || p.CompressionFactor >= 1 ||
		!finiteStrategy(p.VolumeMultiplier) || p.VolumeMultiplier < 1 {
		return fmt.Errorf("strategy: invalid vcb v2 parameters")
	}
	return nil
}

func ValidateMeanReversionV1Params(p MeanReversionV1Params) error {
	if !finiteStrategy(p.OversoldRSI) || p.OversoldRSI <= 0 || p.OversoldRSI >= ResearchV3RSIRecovery ||
		!finiteStrategy(p.StopATR) || p.StopATR <= 0 {
		return fmt.Errorf("strategy: invalid mean reversion parameters")
	}
	return nil
}

// ResearchV3VolatilityCompressionSignals emits 1h close-confirmed breakouts.
// Every comparison window ends before the signal candle, preventing lookahead.
func ResearchV3VolatilityCompressionSignals(minutes []model.Candle, p VolatilityCompressionBreakoutV2Params) ([]EntrySignal, error) {
	if err := ValidateVolatilityCompressionBreakoutV2Params(p); err != nil {
		return nil, err
	}
	candles := AggregateMinutes(minutes, 60)
	minimum := ResearchV3EMAPeriod + ResearchV3EMARiseBars + ResearchV3VolumePeriod + ResearchV3RangeBars + 2
	if len(candles) < minimum {
		return nil, nil
	}
	ema := EMA(candles, ResearchV3EMAPeriod)
	atr := ATRWilder(candles, ResearchV3ATRPeriod)
	volumes := candleVolumes(candles)
	volumeSMA := smaFloats(volumes, ResearchV3VolumePeriod)
	atrSMA := smaFloats(atr, ResearchV3RangeBars)
	var signals []EntrySignal
	for i := minimum - 1; i < len(candles); i++ {
		if ema[i] <= 0 || ema[i-ResearchV3EMARiseBars] <= 0 || atr[i] <= 0 || atrSMA[i-1] <= 0 || volumeSMA[i-1] <= 0 {
			continue
		}
		if candles[i].Close <= ema[i] || ema[i] <= ema[i-ResearchV3EMARiseBars] ||
			atr[i-1] > atrSMA[i-1]*p.CompressionFactor || volumes[i] < volumeSMA[i-1]*p.VolumeMultiplier {
			continue
		}
		high, low, ok := priorRange(candles, i, ResearchV3RangeBars)
		if !ok || candles[i].Close <= high {
			continue
		}
		entry, stop := candles[i].Close, low
		risk := entry - stop
		if risk <= 0 || risk/entry > 0.15 {
			continue
		}
		signals = append(signals, EntrySignal{Time: candles[i].OpenTime, EntryPrice: entry, Stop: stop, TP1: entry + risk, TP2: entry + 2*risk,
			Diagnostics: map[string]float64{"ema200": ema[i], "atr": atr[i], "atr_mean_prior": atrSMA[i-1], "range_high": high, "range_low": low, "volume": volumes[i], "volume_mean_prior": volumeSMA[i-1], "compression_factor": p.CompressionFactor, "volume_multiplier": p.VolumeMultiplier}})
	}
	return signals, nil
}

// ResearchV3MeanReversionSignals emits a recovery only after a prior RSI
// oversold close. The current bar can confirm recovery, but no future candle
// contributes to either the trend, RSI, or stop calculation.
func ResearchV3MeanReversionSignals(minutes []model.Candle, p MeanReversionV1Params) ([]EntrySignal, error) {
	if err := ValidateMeanReversionV1Params(p); err != nil {
		return nil, err
	}
	candles := AggregateMinutes(minutes, 60)
	minimum := ResearchV3EMAPeriod + ResearchV3EMARiseBars + ResearchV3RSIPeriod + 2
	if len(candles) < minimum {
		return nil, nil
	}
	ema := EMA(candles, ResearchV3EMAPeriod)
	atr := ATRWilder(candles, ResearchV3ATRPeriod)
	rsi := RSIWilder(candles, ResearchV3RSIPeriod)
	var signals []EntrySignal
	for i := minimum - 1; i < len(candles); i++ {
		if ema[i] <= 0 || ema[i-ResearchV3EMARiseBars] <= 0 || atr[i] <= 0 || rsi[i-1] <= 0 || rsi[i] <= 0 {
			continue
		}
		if candles[i].Close <= ema[i] || ema[i] <= ema[i-ResearchV3EMARiseBars] ||
			rsi[i-1] > p.OversoldRSI || rsi[i-1] >= ResearchV3RSIRecovery || rsi[i] < ResearchV3RSIRecovery || candles[i].Close <= candles[i-1].Close {
			continue
		}
		entry := candles[i].Close
		stop := math.Min(candles[i].Low, candles[i-1].Low)
		atrStop := entry - atr[i]*p.StopATR
		if atrStop < stop {
			stop = atrStop
		}
		risk := entry - stop
		if risk <= 0 || risk/entry > 0.15 {
			continue
		}
		signals = append(signals, EntrySignal{Time: candles[i].OpenTime, EntryPrice: entry, Stop: stop, TP1: entry + risk, TP2: entry + 2*risk,
			Diagnostics: map[string]float64{"ema200": ema[i], "atr": atr[i], "rsi_prior": rsi[i-1], "rsi_recovery": rsi[i], "oversold_rsi": p.OversoldRSI, "stop_atr": p.StopATR, "stop": stop}})
	}
	return signals, nil
}

func candleVolumes(candles []model.Candle) []float64 {
	values := make([]float64, len(candles))
	for i := range candles {
		values[i] = candles[i].Volume
	}
	return values
}

func priorRange(candles []model.Candle, index, bars int) (float64, float64, bool) {
	if index < bars || bars < 1 {
		return 0, 0, false
	}
	high, low := candles[index-bars].High, candles[index-bars].Low
	for i := index - bars + 1; i < index; i++ {
		if candles[i].High > high {
			high = candles[i].High
		}
		if candles[i].Low < low {
			low = candles[i].Low
		}
	}
	return high, low, high > 0 && low > 0 && high >= low
}

// RSIWilder is zero until its initial period is fully observed.
func RSIWilder(candles []model.Candle, period int) []float64 {
	out := make([]float64, len(candles))
	if period < 1 || len(candles) <= period {
		return out
	}
	gain, loss := 0.0, 0.0
	for i := 1; i <= period; i++ {
		delta := candles[i].Close - candles[i-1].Close
		if delta >= 0 {
			gain += delta
		} else {
			loss -= delta
		}
	}
	averageGain, averageLoss := gain/float64(period), loss/float64(period)
	out[period] = rsiValue(averageGain, averageLoss)
	for i := period + 1; i < len(candles); i++ {
		delta := candles[i].Close - candles[i-1].Close
		currentGain, currentLoss := 0.0, 0.0
		if delta >= 0 {
			currentGain = delta
		} else {
			currentLoss = -delta
		}
		averageGain = (averageGain*float64(period-1) + currentGain) / float64(period)
		averageLoss = (averageLoss*float64(period-1) + currentLoss) / float64(period)
		out[i] = rsiValue(averageGain, averageLoss)
	}
	return out
}

func rsiValue(gain, loss float64) float64 {
	if loss == 0 {
		if gain > 0 {
			return 100
		}
		return 50
	}
	return 100 - 100/(1+gain/loss)
}

func finiteStrategy(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

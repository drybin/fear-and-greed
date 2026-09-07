package strategy

import (
	"fmt"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	EMAPullbackTrendPeriod   = 200
	EMAPullbackTrendRiseBars = 20
	EMAPullbackATRPeriod     = 14
	EMAPullbackRecoveryBars  = 12
	EMAPullbackTimeExit      = 7 * 24 * time.Hour
)

// EMAPullbackV1Params bounds the pullback depth and initial risk alternatives.
// Trend, recovery, target, and timeout behavior are frozen for this version.
type EMAPullbackV1Params struct {
	PullbackEMAPeriod int
	StopATR           float64
}

func ValidateEMAPullbackV1Params(p EMAPullbackV1Params) error {
	if (p.PullbackEMAPeriod != 20 && p.PullbackEMAPeriod != 50) ||
		!finiteStrategy(p.StopATR) || p.StopATR <= 0 {
		return fmt.Errorf("strategy: invalid EMA pullback v1 parameters")
	}
	return nil
}

// EMAPullbackV1Signals detects a completed 1h touch of EMA20 or EMA50 and a
// later green recovery close. The higher-timeframe trend only uses the last
// fully closed 4h candle; the engine fills no earlier than the next 1h open.
func EMAPullbackV1Signals(minutes []model.Candle, p EMAPullbackV1Params) ([]EntrySignal, error) {
	if err := ValidateEMAPullbackV1Params(p); err != nil {
		return nil, err
	}
	return emaPullbackV1Signals(AggregateMinutes(minutes, 60), AggregateMinutes(minutes, 240), p)
}

func emaPullbackV1Signals(hours, fourHours []model.Candle, p EMAPullbackV1Params) ([]EntrySignal, error) {
	if err := ValidateEMAPullbackV1Params(p); err != nil {
		return nil, err
	}
	minimumHours := p.PullbackEMAPeriod + 2
	minimumTrendBars := EMAPullbackTrendPeriod + EMAPullbackTrendRiseBars + 1
	if len(hours) < minimumHours || len(fourHours) < minimumTrendBars {
		return nil, nil
	}
	pullbackEMA := EMA(hours, p.PullbackEMAPeriod)
	atr := ATRWilder(hours, EMAPullbackATRPeriod)
	trendEMA := EMA(fourHours, EMAPullbackTrendPeriod)

	type pullback struct {
		index int
		low   float64
	}
	var active *pullback
	signals := make([]EntrySignal, 0)
	for i := minimumHours - 1; i < len(hours); i++ {
		if active != nil && i-active.index > EMAPullbackRecoveryBars {
			active = nil
		}
		trendIndex := completedTrendIndex(fourHours, hours[i].OpenTime.Add(time.Hour))
		if trendIndex < minimumTrendBars-1 || trendEMA[trendIndex] <= 0 || trendEMA[trendIndex-EMAPullbackTrendRiseBars] <= 0 ||
			fourHours[trendIndex].Close <= trendEMA[trendIndex] || trendEMA[trendIndex] <= trendEMA[trendIndex-EMAPullbackTrendRiseBars] {
			active = nil
			continue
		}
		if pullbackEMA[i] <= 0 || atr[i] <= 0 {
			continue
		}

		if active != nil && i > active.index && hours[i].Close > hours[i].Open && hours[i].Close > pullbackEMA[i] {
			entry := hours[i].Close
			pullbackLow := active.low
			if hours[i].Low < pullbackLow {
				pullbackLow = hours[i].Low
			}
			stop := pullbackLow
			atrStop := entry - p.StopATR*atr[i]
			if atrStop < stop {
				stop = atrStop
			}
			risk := entry - stop
			if stop > 0 && risk > 0 && risk/entry <= .15 {
				signals = append(signals, EntrySignal{
					Time: hours[i].OpenTime, EntryPrice: entry, Stop: stop,
					TP1: entry + risk, TP2: entry + 3*risk,
					TimeExitAt: hours[i].OpenTime.Add(EMAPullbackTimeExit),
					Diagnostics: map[string]float64{
						"pullback_ema": pullbackEMA[i], "pullback_ema_period": float64(p.PullbackEMAPeriod),
						"pullback_low": pullbackLow, "pullback_index": float64(active.index),
						"atr14": atr[i], "stop_atr": p.StopATR,
						"trend_close_4h": fourHours[trendIndex].Close, "trend_ema200_4h": trendEMA[trendIndex],
					},
				})
			}
			active = nil
			continue
		}
		if hours[i].Low <= pullbackEMA[i] && hours[i].High >= pullbackEMA[i] {
			active = &pullback{index: i, low: hours[i].Low}
		}
	}
	return signals, nil
}

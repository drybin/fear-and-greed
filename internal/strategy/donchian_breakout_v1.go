package strategy

import (
	"fmt"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	DonchianBreakoutEMAPeriod   = 200
	DonchianBreakoutEMARiseBars = 20
	DonchianBreakoutATRPeriod   = 14
	DonchianBreakoutTimeExit    = 21 * 24 * time.Hour
)

// DonchianBreakoutV1Params bounds the channel and risk-distance alternatives.
// Trend definition and exits are fixed before the development run.
type DonchianBreakoutV1Params struct {
	ChannelBars int
	StopATR     float64
}

func ValidateDonchianBreakoutV1Params(p DonchianBreakoutV1Params) error {
	if p.ChannelBars < 2 || !finiteStrategy(p.StopATR) || p.StopATR <= 0 {
		return fmt.Errorf("strategy: invalid Donchian breakout v1 parameters")
	}
	return nil
}

// DonchianBreakoutV1Signals emits 4h trend-continuation entries after a close
// crosses a channel high calculated only from earlier completed candles. The
// protocol engine takes the next available 4h open and owns all later exits.
func DonchianBreakoutV1Signals(minutes []model.Candle, p DonchianBreakoutV1Params) ([]EntrySignal, error) {
	if err := ValidateDonchianBreakoutV1Params(p); err != nil {
		return nil, err
	}
	return donchianBreakoutV1Signals(AggregateMinutes(minutes, 240), p)
}

func donchianBreakoutV1Signals(candles []model.Candle, p DonchianBreakoutV1Params) ([]EntrySignal, error) {
	if err := ValidateDonchianBreakoutV1Params(p); err != nil {
		return nil, err
	}
	minimum := DonchianBreakoutEMAPeriod + DonchianBreakoutEMARiseBars + 1
	if p.ChannelBars+1 > minimum {
		minimum = p.ChannelBars + 1
	}
	if len(candles) < minimum {
		return nil, nil
	}
	ema := EMA(candles, DonchianBreakoutEMAPeriod)
	atr := ATRWilder(candles, DonchianBreakoutATRPeriod)
	signals := make([]EntrySignal, 0)
	for i := minimum - 1; i < len(candles); i++ {
		channelHigh, _, ok := priorRange(candles, i, p.ChannelBars)
		if !ok || ema[i] <= 0 || ema[i-DonchianBreakoutEMARiseBars] <= 0 || atr[i] <= 0 {
			continue
		}
		if candles[i].Close <= channelHigh || candles[i-1].Close > channelHigh ||
			candles[i].Close <= ema[i] || ema[i] <= ema[i-DonchianBreakoutEMARiseBars] {
			continue
		}
		entry := candles[i].Close
		stop := entry - p.StopATR*atr[i]
		risk := entry - stop
		if stop <= 0 || risk <= 0 || risk/entry > .15 {
			continue
		}
		signals = append(signals, EntrySignal{
			Time: candles[i].OpenTime, EntryPrice: entry, Stop: stop,
			TP1: entry + risk, TP2: entry + 3*risk,
			TimeExitAt: candles[i].OpenTime.Add(DonchianBreakoutTimeExit),
			Diagnostics: map[string]float64{
				"channel_high": channelHigh, "channel_bars": float64(p.ChannelBars),
				"ema200": ema[i], "ema200_prior": ema[i-DonchianBreakoutEMARiseBars],
				"atr14": atr[i], "stop_atr": p.StopATR,
			},
		})
	}
	return signals, nil
}

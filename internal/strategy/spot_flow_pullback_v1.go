package strategy

import (
	"fmt"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	SpotFlowPullbackTrendPeriod  = 200
	SpotFlowPullbackTrendRise    = 20
	SpotFlowPullbackVolumePeriod = 20
	SpotFlowPullbackLookback     = 3
	SpotFlowPullbackMinBuyShare  = 0.55
	SpotFlowPullbackTimeExit     = 48 * time.Hour
)

// SpotFlowPullbackV1Signals enters a completed 1h recovery after a short
// pullback in a causal rising 4h trend. It requires Binance spot quote volume
// and taker-buy quote volume, which are known only when the signal hour closes.
func SpotFlowPullbackV1Signals(minutes []model.Candle) ([]EntrySignal, error) {
	if err := validateSpotFlow(minutes); err != nil {
		return nil, err
	}
	return spotFlowPullbackV1Signals(AggregateMinutes(minutes, 60), AggregateMinutes(minutes, 240))
}

func spotFlowPullbackV1Signals(hours, fourHours []model.Candle) ([]EntrySignal, error) {
	minimumTrendBars := SpotFlowPullbackTrendPeriod + SpotFlowPullbackTrendRise + 1
	minimumHours := SpotFlowPullbackVolumePeriod + SpotFlowPullbackLookback + 1
	if len(hours) < minimumHours || len(fourHours) < minimumTrendBars {
		return nil, nil
	}
	trendEMA := EMA(fourHours, SpotFlowPullbackTrendPeriod)
	quoteSMA := smaFloats(candleQuoteVolumes(hours), SpotFlowPullbackVolumePeriod)
	signals := make([]EntrySignal, 0)
	for i := minimumHours - 1; i < len(hours); i++ {
		trendIndex := completedTrendIndex(fourHours, hours[i].OpenTime.Add(time.Hour))
		if trendIndex < minimumTrendBars-1 || trendEMA[trendIndex] <= trendEMA[trendIndex-SpotFlowPullbackTrendRise] || fourHours[trendIndex].Close <= trendEMA[trendIndex] {
			continue
		}
		if quoteSMA[i-1] <= 0 || hours[i].QuoteVolume < quoteSMA[i-1] || hours[i].Close <= hours[i].Open || hours[i-1].Close >= hours[i-SpotFlowPullbackLookback].Close {
			continue
		}
		buyShare := hours[i].TakerBuyQuoteVolume / hours[i].QuoteVolume
		if !finiteStrategy(buyShare) || buyShare < SpotFlowPullbackMinBuyShare {
			continue
		}
		entry, stop := hours[i].Close, hours[i].Low
		risk := entry - stop
		if stop <= 0 || risk <= 0 || risk/entry > .15 {
			continue
		}
		signals = append(signals, EntrySignal{
			Time: hours[i].OpenTime, EntryPrice: entry, Stop: stop,
			TP1: entry + risk, TP2: entry + 2*risk, TimeExitAt: hours[i].OpenTime.Add(SpotFlowPullbackTimeExit),
			Diagnostics: map[string]float64{
				"buy_share": buyShare, "min_buy_share": SpotFlowPullbackMinBuyShare,
				"quote_volume": hours[i].QuoteVolume, "quote_volume_sma20_prior": quoteSMA[i-1],
				"pullback_close_3h_ago": hours[i-SpotFlowPullbackLookback].Close,
				"trend_close_4h":        fourHours[trendIndex].Close, "trend_ema200_4h": trendEMA[trendIndex],
			},
		})
	}
	return signals, nil
}

func validateSpotFlow(candles []model.Candle) error {
	for i, candle := range candles {
		if !finiteStrategy(candle.QuoteVolume) || !finiteStrategy(candle.TakerBuyQuoteVolume) || candle.QuoteVolume <= 0 || candle.TakerBuyQuoteVolume < 0 || candle.TakerBuyQuoteVolume > candle.QuoteVolume || candle.Trades <= 0 {
			return fmt.Errorf("strategy: spot-flow candle %d at %s is missing or invalid quote/trade flow", i, candle.OpenTime.UTC().Format(time.RFC3339))
		}
	}
	return nil
}

func candleQuoteVolumes(candles []model.Candle) []float64 {
	values := make([]float64, len(candles))
	for i, candle := range candles {
		values[i] = candle.QuoteVolume
	}
	return values
}

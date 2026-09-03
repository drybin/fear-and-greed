package strategy

import (
	"fmt"
	"math"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	BollingerRangePeriod    = 20
	BollingerRangeADXPeriod = 14
	BollingerRangeATRPeriod = 14
	BollingerRangeStopATR   = 1.5
	BollingerRangeTimeExit  = 48 * time.Hour
)

// BollingerRangeReversionV1Params bounds the range classification and band
// width alternatives. All confirmation and exit behavior remains fixed.
type BollingerRangeReversionV1Params struct {
	ADXMaximum       float64
	BandStdDeviation float64
}

func ValidateBollingerRangeReversionV1Params(p BollingerRangeReversionV1Params) error {
	if !finiteStrategy(p.ADXMaximum) || p.ADXMaximum <= 0 || p.ADXMaximum >= 100 ||
		!finiteStrategy(p.BandStdDeviation) || p.BandStdDeviation <= 0 {
		return fmt.Errorf("strategy: invalid Bollinger range reversion v1 parameters")
	}
	return nil
}

// BollingerRangeReversionV1Signals buys a completed re-entry into the lower
// Bollinger band only while the completed ADX classifies the market as range.
// The preceding excursion and every indicator value are known at signal close.
func BollingerRangeReversionV1Signals(minutes []model.Candle, p BollingerRangeReversionV1Params) ([]EntrySignal, error) {
	if err := ValidateBollingerRangeReversionV1Params(p); err != nil {
		return nil, err
	}
	return bollingerRangeReversionV1Signals(AggregateMinutes(minutes, 60), p)
}

func bollingerRangeReversionV1Signals(candles []model.Candle, p BollingerRangeReversionV1Params) ([]EntrySignal, error) {
	if err := ValidateBollingerRangeReversionV1Params(p); err != nil {
		return nil, err
	}
	minimum := 2*BollingerRangeADXPeriod + 1
	if BollingerRangePeriod+1 > minimum {
		minimum = BollingerRangePeriod + 1
	}
	if len(candles) < minimum {
		return nil, nil
	}
	middle, lower, upper := BollingerBands(candles, BollingerRangePeriod, p.BandStdDeviation)
	adx := ADXWilder(candles, BollingerRangeADXPeriod)
	atr := ATRWilder(candles, BollingerRangeATRPeriod)
	signals := make([]EntrySignal, 0)
	for i := minimum - 1; i < len(candles); i++ {
		if adx[i] <= 0 || adx[i] > p.ADXMaximum || atr[i] <= 0 || middle[i] <= 0 || lower[i] <= 0 || upper[i] <= middle[i] {
			continue
		}
		if candles[i-1].Close >= lower[i-1] || candles[i].Close <= lower[i] || candles[i].Close >= middle[i] {
			continue
		}
		entry := candles[i].Close
		stop := math.Min(candles[i-1].Low, entry-BollingerRangeStopATR*atr[i])
		risk := entry - stop
		if stop <= 0 || risk <= 0 || risk/entry > .15 {
			continue
		}
		signals = append(signals, EntrySignal{
			Time: candles[i].OpenTime, EntryPrice: entry, Stop: stop,
			TP1: middle[i], TP2: upper[i], TimeExitAt: candles[i].OpenTime.Add(BollingerRangeTimeExit),
			Diagnostics: map[string]float64{
				"adx14": adx[i], "adx_maximum": p.ADXMaximum, "band_std_deviation": p.BandStdDeviation,
				"band_middle": middle[i], "band_lower_prior": lower[i-1], "band_lower": lower[i], "band_upper": upper[i],
				"atr14": atr[i], "stop_atr": BollingerRangeStopATR,
			},
		})
	}
	return signals, nil
}

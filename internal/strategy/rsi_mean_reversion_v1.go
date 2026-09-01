package strategy

import (
	"fmt"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

const (
	RSIMeanReversionRSIPeriod        = 14
	RSIMeanReversionRecovery         = 35.0
	RSIMeanReversionEntryEMAPeriod   = 20
	RSIMeanReversionTrendEMAPeriod   = 200
	RSIMeanReversionTrendRiseBars    = 20
	RSIMeanReversionATRPeriod        = 14
	RSIMeanReversionTimeExit         = 48 * time.Hour
	RSIMeanReversionHoursPerTrendBar = 4
)

// RSIMeanReversionV1Params bounds the only two hypothesis thresholds. The
// recovery level, target, trend definition, and time exit are fixed behavior.
type RSIMeanReversionV1Params struct {
	OversoldRSI float64
	StopATR     float64
}

func ValidateRSIMeanReversionV1Params(p RSIMeanReversionV1Params) error {
	if !finiteStrategy(p.OversoldRSI) || p.OversoldRSI <= 0 || p.OversoldRSI >= RSIMeanReversionRecovery ||
		!finiteStrategy(p.StopATR) || p.StopATR <= 0 {
		return fmt.Errorf("strategy: invalid RSI mean-reversion v1 parameters")
	}
	return nil
}

// RSIMeanReversionV1Signals detects a 1h oversold RSI recovery only when the
// latest fully closed 4h candle confirms a rising EMA200 trend. Signals are
// close-confirmed; the protocol-v2 engine fills no earlier than the next hour.
func RSIMeanReversionV1Signals(minutes []model.Candle, p RSIMeanReversionV1Params) ([]EntrySignal, error) {
	if err := ValidateRSIMeanReversionV1Params(p); err != nil {
		return nil, err
	}
	hours := AggregateMinutes(minutes, 60)
	fourHours := AggregateMinutes(minutes, 240)
	return rsiMeanReversionV1Signals(hours, fourHours, p)
}

func rsiMeanReversionV1Signals(hours, fourHours []model.Candle, p RSIMeanReversionV1Params) ([]EntrySignal, error) {
	if err := ValidateRSIMeanReversionV1Params(p); err != nil {
		return nil, err
	}
	minimumHours := RSIMeanReversionEntryEMAPeriod + RSIMeanReversionRSIPeriod + 2
	minimumTrendBars := RSIMeanReversionTrendEMAPeriod + RSIMeanReversionTrendRiseBars + 1
	if len(hours) < minimumHours || len(fourHours) < minimumTrendBars {
		return nil, nil
	}
	entryEMA := EMA(hours, RSIMeanReversionEntryEMAPeriod)
	atr := ATRWilder(hours, RSIMeanReversionATRPeriod)
	rsi := RSIWilder(hours, RSIMeanReversionRSIPeriod)
	trendEMA := EMA(fourHours, RSIMeanReversionTrendEMAPeriod)

	signals := make([]EntrySignal, 0)
	oversoldSeen := false
	for i := minimumHours - 1; i < len(hours); i++ {
		if rsi[i] > 0 && rsi[i] <= p.OversoldRSI {
			oversoldSeen = true
		}
		if !oversoldSeen || i == 0 || rsi[i-1] <= 0 || rsi[i] <= 0 || rsi[i-1] >= RSIMeanReversionRecovery || rsi[i] < RSIMeanReversionRecovery {
			continue
		}

		trendIndex := completedTrendIndex(fourHours, hours[i].OpenTime.Add(time.Hour))
		if trendIndex < RSIMeanReversionTrendEMAPeriod+RSIMeanReversionTrendRiseBars || trendEMA[trendIndex] <= 0 || trendEMA[trendIndex-RSIMeanReversionTrendRiseBars] <= 0 {
			continue
		}
		trend := fourHours[trendIndex]
		if trend.Close <= trendEMA[trendIndex] || trendEMA[trendIndex] <= trendEMA[trendIndex-RSIMeanReversionTrendRiseBars] {
			continue
		}
		if entryEMA[i] <= 0 || atr[i] <= 0 || hours[i].Close >= entryEMA[i] {
			continue
		}
		entry, target := hours[i].Close, entryEMA[i]
		stop := entry - atr[i]*p.StopATR
		if stop <= 0 || stop >= entry || target <= entry || (entry-stop)/entry > .15 {
			continue
		}
		signals = append(signals, EntrySignal{
			Time: hours[i].OpenTime, EntryPrice: entry, Stop: stop, TP1: target, TP2: target,
			ExitAllAtTP1: true, TimeExitAt: hours[i].OpenTime.Add(RSIMeanReversionTimeExit),
			Diagnostics: map[string]float64{
				"rsi_prior": rsi[i-1], "rsi_recovery": rsi[i], "oversold_rsi": p.OversoldRSI,
				"entry_ema20": entryEMA[i], "atr14": atr[i], "stop_atr": p.StopATR,
				"trend_close_4h": trend.Close, "trend_ema200_4h": trendEMA[trendIndex],
			},
		})
		oversoldSeen = false
	}
	return signals, nil
}

// completedTrendIndex returns the last 4h candle that closed before the 1h
// confirmation closes. It deliberately excludes the currently forming 4h bar.
func completedTrendIndex(fourHours []model.Candle, hourClose time.Time) int {
	index := -1
	for i, candle := range fourHours {
		if candle.OpenTime.Add(RSIMeanReversionHoursPerTrendBar * time.Hour).After(hourClose) {
			break
		}
		index = i
	}
	return index
}

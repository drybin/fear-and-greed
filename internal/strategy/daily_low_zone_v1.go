package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

// DailyLowZoneSignals evaluates 15-minute candles using only completed daily
// bars. For each day it forms an entry zone from yesterday's low down to the
// first earlier daily low that is strictly lower. The deadline is the open of
// the day after one full additional holding day has elapsed.
func DailyLowZoneSignals(minutes []model.Candle) []EntrySignal {
	candles := AggregateMinutes(minutes, 15)
	if len(candles) == 0 {
		return nil
	}
	days := dailyRanges(candles)
	var signals []EntrySignal
	for dayIndex := 1; dayIndex < len(days); dayIndex++ {
		previous := days[dayIndex-1]
		upper, target := dayLow(candles, previous), dayHigh(candles, previous)
		lower, ok := priorLowerDailyLow(candles, days[:dayIndex-1], upper)
		if !ok || lower <= 0 || upper <= lower || target <= upper {
			continue
		}
		zoneVisited, zoneInvalidated := false, false
		for i := days[dayIndex].start; i < days[dayIndex].end; i++ {
			candle := candles[i]
			visitedBeforeCandle := zoneVisited
			if candle.Low < lower {
				zoneInvalidated = true
				continue
			}
			if candle.Low <= upper && candle.High >= lower {
				zoneVisited = true
			}
			// The confirmation must follow an already observed zone touch. Using
			// a completed green candle avoids assuming the intrabar price path.
			if !visitedBeforeCandle || zoneInvalidated || candle.Open > upper || candle.Close <= upper || candle.Close <= candle.Open {
				continue
			}
			dayStart := candle.OpenTime.UTC().Truncate(24 * time.Hour)
			signals = append(signals, EntrySignal{
				Time: candle.OpenTime, EntryPrice: candle.Close, Stop: lower, TP1: target, TP2: target,
				ExitAllAtTP1: true, TimeExitAt: dayStart.AddDate(0, 0, 2),
				Diagnostics: map[string]float64{"zone_low": lower, "zone_high": upper, "previous_day_high": target},
			})
			break // One planned entry per daily zone.
		}
	}
	return signals
}

type dailyRange struct{ start, end int }

func dailyRanges(candles []model.Candle) []dailyRange {
	starts := []int{0}
	for i := 1; i < len(candles); i++ {
		if !sameUTCDay(candles[i-1].OpenTime, candles[i].OpenTime) {
			starts = append(starts, i)
		}
	}
	ranges := make([]dailyRange, 0, len(starts))
	for i, start := range starts {
		end := len(candles)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		ranges = append(ranges, dailyRange{start: start, end: end})
	}
	return ranges
}

func priorLowerDailyLow(candles []model.Candle, days []dailyRange, upper float64) (float64, bool) {
	for i := len(days) - 1; i >= 0; i-- {
		low := dayLow(candles, days[i])
		if low < upper {
			return low, true
		}
	}
	return 0, false
}

func dayLow(candles []model.Candle, day dailyRange) float64 {
	low := candles[day.start].Low
	for i := day.start + 1; i < day.end; i++ {
		if candles[i].Low < low {
			low = candles[i].Low
		}
	}
	return low
}

func dayHigh(candles []model.Candle, day dailyRange) float64 {
	high := 0.0
	for i := day.start; i < day.end; i++ {
		if candles[i].High > high {
			high = candles[i].High
		}
	}
	return high
}

func sameUTCDay(left, right time.Time) bool {
	return left.UTC().Truncate(24 * time.Hour).Equal(right.UTC().Truncate(24 * time.Hour))
}

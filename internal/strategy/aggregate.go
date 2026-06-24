package strategy

import (
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

// AggregateMinutes builds higher-TF candles from minute (or uniform step) series.
// Bucket open time is UTC-aligned; volume is summed.
func AggregateMinutes(candles []model.Candle, minutes int) []model.Candle {
	if len(candles) == 0 || minutes < 1 {
		return nil
	}
	var out []model.Candle
	var cur *model.Candle
	var curBucket time.Time

	flush := func() {
		if cur != nil {
			out = append(out, *cur)
			cur = nil
		}
	}

	for _, c := range candles {
		b := bucketOpenUTC(c.OpenTime, minutes)
		if cur == nil || !b.Equal(curBucket) {
			flush()
			curBucket = b
			cur = &model.Candle{
				OpenTime: b,
				Open:     c.Open,
				High:     c.High,
				Low:      c.Low,
				Close:    c.Close,
				Volume:   c.Volume,
			}
			continue
		}
		if c.High > cur.High {
			cur.High = c.High
		}
		if c.Low < cur.Low {
			cur.Low = c.Low
		}
		cur.Close = c.Close
		cur.Volume += c.Volume
	}
	flush()
	return out
}

func bucketOpenUTC(t time.Time, minutes int) time.Time {
	t = t.UTC()
	if minutes == 240 {
		h := (t.Hour() / 4) * 4
		return time.Date(t.Year(), t.Month(), t.Day(), h, 0, 0, 0, time.UTC)
	}
	if minutes == 60 {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC)
	}
	if minutes == 15 {
		m := (t.Minute() / 15) * 15
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), m, 0, 0, time.UTC)
	}
	// generic: floor to minutes from midnight
	totalMin := t.Hour()*60 + t.Minute()
	floor := (totalMin / minutes) * minutes
	return time.Date(t.Year(), t.Month(), t.Day(), floor/60, floor%60, 0, 0, time.UTC)
}

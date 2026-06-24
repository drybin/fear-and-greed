package strategy

import "github.com/drybin/fear-and-greed/internal/domain/model"

func candleRange(c model.Candle) float64 {
	return c.High - c.Low
}

// ATRWilder returns ATR(period) per bar index; zero where not enough history.
func ATRWilder(candles []model.Candle, period int) []float64 {
	out := make([]float64, len(candles))
	if period < 1 || len(candles) == 0 {
		return out
	}
	tr := make([]float64, len(candles))
	for i := range candles {
		if i == 0 {
			tr[i] = candles[i].High - candles[i].Low
			continue
		}
		hl := candles[i].High - candles[i].Low
		hc := abs(candles[i].High - candles[i-1].Close)
		lc := abs(candles[i].Low - candles[i-1].Close)
		tr[i] = max3(hl, hc, lc)
	}
	if len(candles) < period {
		return out
	}
	var sum float64
	for i := 0; i < period; i++ {
		sum += tr[i]
	}
	out[period-1] = sum / float64(period)
	for i := period; i < len(candles); i++ {
		out[i] = (out[i-1]*float64(period-1) + tr[i]) / float64(period)
	}
	return out
}

func smaFloats(vals []float64, period int) []float64 {
	out := make([]float64, len(vals))
	if period < 1 {
		return out
	}
	var sum float64
	for i := 0; i < len(vals); i++ {
		sum += vals[i]
		if i >= period {
			sum -= vals[i-period]
		}
		if i >= period-1 {
			out[i] = sum / float64(period)
		}
	}
	return out
}

func maxHighBefore(candles []model.Candle, i, lookback int) float64 {
	start := i - lookback
	if start < 0 {
		start = 0
	}
	var m float64
	for j := start; j < i; j++ {
		if candles[j].High > m {
			m = candles[j].High
		}
	}
	return m
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

// EMA returns exponential moving average of close; zero until period-1 (seeded with SMA).
func EMA(candles []model.Candle, period int) []float64 {
	out := make([]float64, len(candles))
	if period < 1 || len(candles) < period {
		return out
	}
	mult := 2.0 / float64(period+1)
	var sum float64
	for i := 0; i < period; i++ {
		sum += candles[i].Close
	}
	out[period-1] = sum / float64(period)
	for i := period; i < len(candles); i++ {
		out[i] = (candles[i].Close-out[i-1])*mult + out[i-1]
	}
	return out
}

func max3(a, b, c float64) float64 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

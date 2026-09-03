package strategy

import (
	"math"

	"github.com/drybin/fear-and-greed/internal/domain/model"
)

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

// BollingerBands returns SMA-based bands. All values at index i use candles
// through i only, so a close can causally confirm a re-entry into a band.
func BollingerBands(candles []model.Candle, period int, multiplier float64) (middle, lower, upper []float64) {
	middle = make([]float64, len(candles))
	lower = make([]float64, len(candles))
	upper = make([]float64, len(candles))
	if period < 2 || multiplier <= 0 {
		return middle, lower, upper
	}
	for i := period - 1; i < len(candles); i++ {
		var sum float64
		for j := i - period + 1; j <= i; j++ {
			sum += candles[j].Close
		}
		mean := sum / float64(period)
		var squaredDistance float64
		for j := i - period + 1; j <= i; j++ {
			delta := candles[j].Close - mean
			squaredDistance += delta * delta
		}
		deviation := math.Sqrt(squaredDistance / float64(period))
		middle[i] = mean
		lower[i] = mean - multiplier*deviation
		upper[i] = mean + multiplier*deviation
	}
	return middle, lower, upper
}

// ADXWilder returns the standard Wilder ADX. It is zero until both the DI and
// ADX smoothing windows are complete.
func ADXWilder(candles []model.Candle, period int) []float64 {
	out := make([]float64, len(candles))
	if period < 2 || len(candles) < 2*period {
		return out
	}
	tr := make([]float64, len(candles))
	plusDM := make([]float64, len(candles))
	minusDM := make([]float64, len(candles))
	for i := 1; i < len(candles); i++ {
		tr[i] = max3(candles[i].High-candles[i].Low, abs(candles[i].High-candles[i-1].Close), abs(candles[i].Low-candles[i-1].Close))
		upMove := candles[i].High - candles[i-1].High
		downMove := candles[i-1].Low - candles[i].Low
		if upMove > downMove && upMove > 0 {
			plusDM[i] = upMove
		}
		if downMove > upMove && downMove > 0 {
			minusDM[i] = downMove
		}
	}
	var smoothedTR, smoothedPlus, smoothedMinus float64
	for i := 1; i <= period; i++ {
		smoothedTR += tr[i]
		smoothedPlus += plusDM[i]
		smoothedMinus += minusDM[i]
	}
	dx := make([]float64, len(candles))
	dx[period] = directionalIndex(smoothedTR, smoothedPlus, smoothedMinus)
	for i := period + 1; i < len(candles); i++ {
		smoothedTR = smoothedTR - smoothedTR/float64(period) + tr[i]
		smoothedPlus = smoothedPlus - smoothedPlus/float64(period) + plusDM[i]
		smoothedMinus = smoothedMinus - smoothedMinus/float64(period) + minusDM[i]
		dx[i] = directionalIndex(smoothedTR, smoothedPlus, smoothedMinus)
	}
	firstADX := 2*period - 1
	var sumDX float64
	for i := period; i <= firstADX; i++ {
		sumDX += dx[i]
	}
	out[firstADX] = sumDX / float64(period)
	for i := firstADX + 1; i < len(candles); i++ {
		out[i] = (out[i-1]*float64(period-1) + dx[i]) / float64(period)
	}
	return out
}

func directionalIndex(smoothedTR, smoothedPlus, smoothedMinus float64) float64 {
	if smoothedTR <= 0 {
		return 0
	}
	plusDI := 100 * smoothedPlus / smoothedTR
	minusDI := 100 * smoothedMinus / smoothedTR
	denominator := plusDI + minusDI
	if denominator <= 0 {
		return 0
	}
	return 100 * abs(plusDI-minusDI) / denominator
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

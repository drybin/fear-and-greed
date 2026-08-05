package protocolv2

import "math"

// Money rounding conventions for protocol-v2 accounting.
// Quantities and notionals use fixed decimal places so repeated runs are
// byte-stable across platforms.

const (
	PriceDecimals    = 8
	QuantityDecimals = 8
	FeeDecimals      = 8
	MetricDecimals   = 10
)

func roundTo(v float64, decimals int) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return v
	}
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}

// RoundPrice rounds an execution or mark price.
func RoundPrice(v float64) float64 { return roundTo(v, PriceDecimals) }

// RoundQuantity rounds a position quantity.
func RoundQuantity(v float64) float64 { return roundTo(v, QuantityDecimals) }

// RoundFee rounds commission or slippage cash amounts.
func RoundFee(v float64) float64 { return roundTo(v, FeeDecimals) }

// RoundMetric rounds reported research metrics (returns, ratios).
func RoundMetric(v float64) float64 { return roundTo(v, MetricDecimals) }

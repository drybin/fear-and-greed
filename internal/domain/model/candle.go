package model

import "time"

// Candle is one OHLCV row from fetch-data CSV.
type Candle struct {
	OpenTime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

package model

import "time"

// Candle is one OHLCV and spot-flow row from fetch-data CSV.
type Candle struct {
	OpenTime            time.Time
	Open                float64
	High                float64
	Low                 float64
	Close               float64
	Volume              float64
	QuoteVolume         float64
	Trades              int64
	TakerBuyBaseVolume  float64
	TakerBuyQuoteVolume float64
}

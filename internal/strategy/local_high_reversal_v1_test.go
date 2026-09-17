package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestLocalHighReversalV1SignalsIsCausal(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]model.Candle, 22)
	for i := range candles {
		candles[i] = model.Candle{OpenTime: start.Add(time.Duration(i) * time.Hour), Open: 100, High: 101, Low: 99, Close: 100}
	}
	candles[20] = model.Candle{OpenTime: start.Add(20 * time.Hour), Open: 105, High: 110, Low: 104, Close: 108}
	signals := LocalHighReversalV1Signals(candles)
	require.Len(t, signals, 1)
	require.Equal(t, candles[20].OpenTime, signals[0].Time)
	require.Equal(t, 110.0, signals[0].Stop)
	require.Less(t, signals[0].TP1, signals[0].EntryPrice)
}

package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestLocalLowReversalV1Signals(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	hours := make([]model.Candle, 21)
	for i := range hours {
		hours[i] = model.Candle{OpenTime: start.Add(time.Duration(i) * time.Hour), Open: 100, High: 101, Low: 99, Close: 100}
	}
	hours[20] = model.Candle{OpenTime: hours[20].OpenTime, Open: 100, High: 101, Low: 95, Close: 100}
	signals := LocalLowReversalV1Signals(hours)
	require.Len(t, signals, 1)
	require.Equal(t, 95.0, signals[0].Stop)
	require.Equal(t, 105.0, signals[0].TP1)
	require.Equal(t, 110.0, signals[0].TP2)
}

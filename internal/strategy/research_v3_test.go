package strategy

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/stretchr/testify/require"
)

func TestResearchV3ParameterValidation(t *testing.T) {
	require.Error(t, ValidateVolatilityCompressionBreakoutV2Params(VolatilityCompressionBreakoutV2Params{CompressionFactor: 1, VolumeMultiplier: 1.2}))
	require.Error(t, ValidateMeanReversionV1Params(MeanReversionV1Params{OversoldRSI: 35, StopATR: 1.2}))
	require.NoError(t, ValidateVolatilityCompressionBreakoutV2Params(VolatilityCompressionBreakoutV2Params{CompressionFactor: .65, VolumeMultiplier: 1.2}))
	require.NoError(t, ValidateMeanReversionV1Params(MeanReversionV1Params{OversoldRSI: 25, StopATR: 1.2}))
}

func TestPriorRangeExcludesSignalCandle(t *testing.T) {
	candles := []model.Candle{{High: 12, Low: 8}, {High: 14, Low: 9}, {High: 99, Low: 10}}
	high, low, ok := priorRange(candles, 2, 2)
	require.True(t, ok)
	require.Equal(t, 14.0, high)
	require.Equal(t, 8.0, low)
}

func TestRSIWilderDoesNotSeedBeforeFullPeriod(t *testing.T) {
	rsi := RSIWilder(makeResearchV3Candles(20), 14)
	require.Zero(t, rsi[13])
	require.NotZero(t, rsi[14])
}

func TestResearchV3SignalsNeedEnoughHistory(t *testing.T) {
	candles := makeResearchV3Candles(200 * 60)
	vcb, err := ResearchV3VolatilityCompressionSignals(candles, VolatilityCompressionBreakoutV2Params{CompressionFactor: .65, VolumeMultiplier: 1.2})
	require.NoError(t, err)
	require.Empty(t, vcb)
	mean, err := ResearchV3MeanReversionSignals(candles, MeanReversionV1Params{OversoldRSI: 25, StopATR: 1.2})
	require.NoError(t, err)
	require.Empty(t, mean)
}

func makeResearchV3Candles(count int) []model.Candle {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	result := make([]model.Candle, count)
	for i := range result {
		price := 100.0 + float64(i)/1000
		result[i] = model.Candle{OpenTime: start.Add(time.Duration(i) * time.Minute), Open: price, High: price + .2, Low: price - .2, Close: price + .05, Volume: 10}
	}
	return result
}

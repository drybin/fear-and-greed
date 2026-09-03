package manifest_test

import (
	"testing"

	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func TestValidateCoreStrategyCodesRejectsDeferredStrategies(t *testing.T) {
	strategies := []manifest.Strategy{
		{Ref: protocolv2.StrategyRef{Code: "fib-pullback-trend-v1", Version: "v1.0.0"}},
		{Ref: protocolv2.StrategyRef{Code: "nr7-trend-breakout-v1", Version: "v1.0.0"}},
		{Ref: protocolv2.StrategyRef{Code: "volatility-compression-breakout-v1", Version: "v1.0.0"}},
		{Ref: protocolv2.StrategyRef{Code: "breakout-retest-long-v2", Version: "v2.0.0"}},
	}
	require.NoError(t, manifest.ValidateCoreStrategyCodes(strategies))

	strategies[3].Ref.Code = "mean-reversion-v1"
	require.ErrorContains(t, manifest.ValidateCoreStrategyCodes(strategies), "outside core validation scope")
}

func TestValidateResearchV3StrategyCodesRejectsMixedSuite(t *testing.T) {
	v3 := []manifest.Strategy{
		{Ref: protocolv2.StrategyRef{Code: "volatility-compression-breakout-v2", Version: "v2.0.0"}},
		{Ref: protocolv2.StrategyRef{Code: "mean-reversion-v1", Version: "v1.0.0"}},
		{Ref: protocolv2.StrategyRef{Code: "daily-low-zone-v1", Version: "v1.0.0"}},
	}
	require.NoError(t, manifest.ValidateResearchV3StrategyCodes(v3))
	require.NoError(t, manifest.ValidateSupportedStrategyCodes(v3))
	v3[1].Ref.Code = "breakout-retest-long-v2"
	require.Error(t, manifest.ValidateSupportedStrategyCodes(v3))
}

func TestValidateDailyLowZoneV11StrategyCodesRejectsMixedSuite(t *testing.T) {
	daily := []manifest.Strategy{{Ref: protocolv2.StrategyRef{Code: "daily-low-zone-v1", Version: "v1.1.0"}}}
	require.NoError(t, manifest.ValidateDailyLowZoneV11StrategyCodes(daily))
	require.NoError(t, manifest.ValidateSupportedStrategyCodes(daily))
	daily[0].Ref.Code = "mean-reversion-v1"
	require.Error(t, manifest.ValidateDailyLowZoneV11StrategyCodes(daily))
}

func TestValidateDailyLowZoneV12StrategyCodesRejectsMixedSuite(t *testing.T) {
	daily := []manifest.Strategy{{Ref: protocolv2.StrategyRef{Code: "daily-low-zone-v1", Version: "v1.2.0"}}}
	require.NoError(t, manifest.ValidateDailyLowZoneV12StrategyCodes(daily))
	require.NoError(t, manifest.ValidateSupportedStrategyCodes(daily))
	daily[0].Ref.Code = "mean-reversion-v1"
	require.Error(t, manifest.ValidateDailyLowZoneV12StrategyCodes(daily))
}

func TestValidateDailyLowZoneV13StrategyCodesRejectsMixedSuite(t *testing.T) {
	daily := []manifest.Strategy{{Ref: protocolv2.StrategyRef{Code: "daily-low-zone-v1", Version: "v1.3.0"}}}
	require.NoError(t, manifest.ValidateDailyLowZoneV13StrategyCodes(daily))
	require.NoError(t, manifest.ValidateSupportedStrategyCodes(daily))
	daily[0].Ref.Code = "mean-reversion-v1"
	require.Error(t, manifest.ValidateDailyLowZoneV13StrategyCodes(daily))
}

func TestValidateRSIMeanReversionV1StrategyCodesRejectsMixedSuite(t *testing.T) {
	rsi := []manifest.Strategy{{Ref: protocolv2.StrategyRef{Code: "rsi-mean-reversion-long-v1", Version: "v1.0.0"}}}
	require.NoError(t, manifest.ValidateRSIMeanReversionV1StrategyCodes(rsi))
	require.NoError(t, manifest.ValidateSupportedStrategyCodes(rsi))
	rsi[0].Ref.Code = "mean-reversion-v1"
	require.Error(t, manifest.ValidateRSIMeanReversionV1StrategyCodes(rsi))
}

func TestValidateDonchianBreakoutV1StrategyCodesRejectsMixedSuite(t *testing.T) {
	donchian := []manifest.Strategy{{Ref: protocolv2.StrategyRef{Code: "donchian-breakout-long-v1", Version: "v1.0.0"}}}
	require.NoError(t, manifest.ValidateDonchianBreakoutV1StrategyCodes(donchian))
	require.NoError(t, manifest.ValidateSupportedStrategyCodes(donchian))
	donchian[0].Ref.Code = "mean-reversion-v1"
	require.Error(t, manifest.ValidateDonchianBreakoutV1StrategyCodes(donchian))
}

func TestValidateBollingerRangeReversionV1StrategyCodesRejectsMixedSuite(t *testing.T) {
	bollinger := []manifest.Strategy{{Ref: protocolv2.StrategyRef{Code: "bollinger-range-reversion-long-v1", Version: "v1.0.0"}}}
	require.NoError(t, manifest.ValidateBollingerRangeReversionV1StrategyCodes(bollinger))
	require.NoError(t, manifest.ValidateSupportedStrategyCodes(bollinger))
	bollinger[0].Ref.Code = "mean-reversion-v1"
	require.Error(t, manifest.ValidateBollingerRangeReversionV1StrategyCodes(bollinger))
}

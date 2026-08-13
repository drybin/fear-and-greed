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
	}
	require.NoError(t, manifest.ValidateResearchV3StrategyCodes(v3))
	require.NoError(t, manifest.ValidateSupportedStrategyCodes(v3))
	v3[1].Ref.Code = "breakout-retest-long-v2"
	require.Error(t, manifest.ValidateSupportedStrategyCodes(v3))
}

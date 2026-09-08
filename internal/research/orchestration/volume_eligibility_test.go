package orchestration

import (
	"testing"

	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/stretchr/testify/require"
)

func TestRequiresValidatedVolumeIncludesBothVolumeStrategies(t *testing.T) {
	require.True(t, requiresValidatedVolume(protocolv2.StrategyRef{Code: "capitulation-reversal-long-v1"}))
	require.True(t, requiresValidatedVolume(protocolv2.StrategyRef{Code: "volume-breakout-long-v1"}))
	require.False(t, requiresValidatedVolume(protocolv2.StrategyRef{Code: "donchian-breakout-long-v1"}))
}

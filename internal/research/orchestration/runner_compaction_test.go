package orchestration

import (
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/metrics"
	"github.com/stretchr/testify/require"
)

func TestCompactionRetainsEndpointsAndSamplesStrategyIntervals(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	equity := make([]execution.EquitySnapshot, 20)
	drawdown := make([]metrics.DrawdownPoint, 20)
	for i := range equity {
		at := start.Add(time.Duration(i) * time.Minute)
		equity[i] = execution.EquitySnapshot{Time: at, TotalEquity: float64(i)}
		drawdown[i] = metrics.DrawdownPoint{Time: at, Equity: float64(i)}
	}

	compactedEquity := compactEquity(equity, 15*time.Minute)
	compactedDrawdown := compactDrawdown(drawdown, 15*time.Minute)
	require.Equal(t, []time.Time{start, start.Add(14 * time.Minute), start.Add(19 * time.Minute)}, equityTimes(compactedEquity))
	require.Equal(t, []time.Time{start, start.Add(14 * time.Minute), start.Add(19 * time.Minute)}, drawdownTimes(compactedDrawdown))

	summary := compactMetrics(metrics.Summary{TradeCount: 7, Drawdown: drawdown}, 15*time.Minute)
	require.Equal(t, 7, summary.TradeCount)
	require.Equal(t, compactedDrawdown, summary.Drawdown)
}

func equityTimes(points []execution.EquitySnapshot) []time.Time {
	out := make([]time.Time, len(points))
	for i, point := range points {
		out[i] = point.Time
	}
	return out
}

func drawdownTimes(points []metrics.DrawdownPoint) []time.Time {
	out := make([]time.Time, len(points))
	for i, point := range points {
		out[i] = point.Time
	}
	return out
}

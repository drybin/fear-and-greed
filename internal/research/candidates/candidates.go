// Package candidates adapts the four frozen legacy core strategies to
// protocol-v2 close-confirmed signals. It intentionally owns no execution,
// sizing, accounting, fold, test-period, or per-symbol selection policy.
package candidates

import (
	"fmt"
	"math"
	"sort"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

const (
	FibPullbackTrendCode              protocolv2.StrategyCode = "fib-pullback-trend-v1"
	NR7TrendBreakoutCode              protocolv2.StrategyCode = "nr7-trend-breakout-v1"
	VolatilityCompressionBreakoutCode protocolv2.StrategyCode = "volatility-compression-breakout-v1"
	BreakoutRetestLongCode            protocolv2.StrategyCode = "breakout-retest-long-v2"
)

// ParameterCandidate is a frozen adapter grid point. Orchestration selects
// exactly one grid point globally from train-only evidence.
type ParameterCandidate struct {
	ID     protocolv2.ParameterCandidateID
	Values map[string]any
}

// Adapter is both a registry strategy and a deterministic signal source.
// Candles must be chronological primary-source OHLCV candles. Signals are
// decisions made on their source-bar close and are executed by Engine later.
type Adapter interface {
	execution.Strategy
	Grid() []ParameterCandidate
	Signals(protocolv2.Symbol, []model.Candle, protocolv2.ParameterCandidateID) ([]execution.CloseConfirmedSignal, error)
}

type adapter struct {
	metadata execution.StrategyMetadata
	grid     []ParameterCandidate
	evaluate func(protocolv2.ParameterCandidateID, []model.Candle) ([]strategy.EntrySignal, error)
}

func (a adapter) Metadata() execution.StrategyMetadata { return a.metadata }

func (a adapter) Grid() []ParameterCandidate {
	out := make([]ParameterCandidate, len(a.grid))
	for i, p := range a.grid {
		out[i] = ParameterCandidate{ID: p.ID, Values: cloneValues(p.Values)}
	}
	return out
}

func (a adapter) Signals(symbol protocolv2.Symbol, candles []model.Candle, candidate protocolv2.ParameterCandidateID) ([]execution.CloseConfirmedSignal, error) {
	if err := symbol.Validate(); err != nil {
		return nil, err
	}
	if !a.hasCandidate(candidate) {
		return nil, fmt.Errorf("candidates: %s has no candidate %q", a.metadata.Ref, candidate)
	}
	entries, err := a.evaluate(candidate, candles)
	if err != nil {
		return nil, err
	}
	out := make([]execution.CloseConfirmedSignal, 0, len(entries))
	for i, entry := range entries {
		signal, err := a.signal(symbol, candidate, i, entry)
		if err != nil {
			return nil, err
		}
		out = append(out, signal)
	}
	return out, nil
}

func (a adapter) hasCandidate(id protocolv2.ParameterCandidateID) bool {
	for _, candidate := range a.grid {
		if candidate.ID == id {
			return true
		}
	}
	return false
}

func (a adapter) signal(symbol protocolv2.Symbol, candidate protocolv2.ParameterCandidateID, index int, entry strategy.EntrySignal) (execution.CloseConfirmedSignal, error) {
	if entry.Time.IsZero() || entry.Stop <= 0 || !finite(entry.Stop) {
		return execution.CloseConfirmedSignal{}, fmt.Errorf("candidates: %s emitted an invalid legacy stop", a.metadata.Ref)
	}
	diagnostics := cloneDiagnostics(entry.Diagnostics)
	diagnostics["legacy_entry_price"] = entry.EntryPrice
	diagnostics["candidate_index"] = float64(index)
	diagnostics["candidate_id_hash"] = stableIDValue(candidate)
	signal := execution.CloseConfirmedSignal{
		SignalID: fmt.Sprintf("%s-%s-%d-%d", a.metadata.Ref.Code, candidate, index, entry.Time.Unix()),
		Strategy: a.metadata.Ref, Symbol: symbol, Timeframe: a.metadata.Timeframe,
		SourceCandleTime: entry.Time.UTC(), Side: execution.SideLong,
		Stop: protocolv2.RoundPrice(entry.Stop),
		Targets: []execution.Target{
			{Name: "tp1", Price: protocolv2.RoundPrice(entry.TP1)},
			{Name: "tp2", Price: protocolv2.RoundPrice(entry.TP2)},
		},
		Diagnostics: diagnostics,
	}
	if err := signal.Validate(); err != nil {
		return execution.CloseConfirmedSignal{}, fmt.Errorf("candidates: %s signal: %w", a.metadata.Ref, err)
	}
	return signal, nil
}

// Core returns the complete, deliberately closed candidate set.
func Core() []Adapter {
	return []Adapter{fib(), nr7(), vcb(), breakoutRetest()}
}

// RegisterCore installs exactly the four scope-approved adapters.
func RegisterCore(registry *execution.Registry) error {
	if registry == nil {
		return fmt.Errorf("candidates: strategy registry is required")
	}
	for _, candidate := range Core() {
		if err := registry.Register(candidate); err != nil {
			return err
		}
	}
	return nil
}

func fib() Adapter {
	grid := make([]ParameterCandidate, 0, 3)
	for i, zone := range strategy.FibPullbackTrendV1SweepZones() {
		id := protocolv2.ParameterCandidateID(fmt.Sprintf("fib-zone-%d", i+1))
		grid = append(grid, ParameterCandidate{ID: id, Values: map[string]any{
			"pivot_length": 5, "min_impulse_pct": 8.0, "zone_top": zone.TopRatio,
			"zone_bottom": zone.BottomRatio, "max_wait_bars_15m": 48,
		}})
	}
	return adapter{
		metadata: execution.StrategyMetadata{Ref: ref(FibPullbackTrendCode, "v1.0.0"), Name: "Fib Pullback Trend v1", Timeframe: "15m", WarmupBars: 918, Description: "Legacy 1h BOS plus 15m fib retest signal adapter."},
		grid:     grid,
		evaluate: func(id protocolv2.ParameterCandidateID, candles []model.Candle) ([]strategy.EntrySignal, error) {
			index := int(id[len("fib-zone-")] - '1')
			zones := strategy.FibPullbackTrendV1SweepZones()
			if index < 0 || index >= len(zones) {
				return nil, fmt.Errorf("unknown fib candidate %q", id)
			}
			report := strategy.SimulateFibPullbackTrendV1WithParams(candles, strategy.FibPullbackTrendV1Params{PivotLength: 5, MinImpulsePct: 8, Zone: zones[index], MaxWaitBars15M: 48})
			return report.Signals, nil
		},
	}
}

func nr7() Adapter {
	filters := strategy.NR7TrendBreakoutV1SweepTrendFilters()
	grid := make([]ParameterCandidate, 0, len(filters))
	for i, filter := range filters {
		grid = append(grid, ParameterCandidate{ID: protocolv2.ParameterCandidateID(fmt.Sprintf("nr7-filter-%d", i+1)), Values: map[string]any{"nr_length": 7, "atr_compression": 0.8, "setup_lifetime": 12, "trend_filter": int(filter)}})
	}
	return adapter{
		metadata: execution.StrategyMetadata{Ref: ref(NR7TrendBreakoutCode, "v1.0.0"), Name: "NR7 Trend Breakout v1", Timeframe: "1h", WarmupBars: 249, Description: "Legacy 1h NR7 compression breakout signal adapter."},
		grid:     grid,
		evaluate: func(id protocolv2.ParameterCandidateID, candles []model.Candle) ([]strategy.EntrySignal, error) {
			index := int(id[len("nr7-filter-")] - '1')
			if index < 0 || index >= len(filters) {
				return nil, fmt.Errorf("unknown nr7 candidate %q", id)
			}
			report := strategy.SimulateNR7TrendBreakoutV1WithParams(candles, strategy.NR7TrendBreakoutV1Params{NRLength: 7, ATRCompression: 0.8, SetupLifetime: 12, TrendFilter: filters[index]})
			return report.Signals, nil
		},
	}
}

func vcb() Adapter {
	grid := make([]ParameterCandidate, 0, 24)
	for _, window := range []int{50, 100} {
		for _, rangeWindow := range []int{5, 10} {
			for _, compression := range []float64{0.5, 0.6, 0.7} {
				for _, expansion := range []float64{1.2, 1.5} {
					id := protocolv2.ParameterCandidateID(fmt.Sprintf("vcb-%02d", len(grid)+1))
					grid = append(grid, ParameterCandidate{ID: id, Values: map[string]any{"compression_window": window, "range_window": rangeWindow, "atr_compression": compression, "breakout_expansion": expansion, "trend_filter": int(strategy.NR7TrendCloseEMA200), "stop_mode": int(strategy.VCBStopCompressionLow)}})
				}
			}
		}
	}
	return adapter{
		metadata: execution.StrategyMetadata{Ref: ref(VolatilityCompressionBreakoutCode, "v1.0.0"), Name: "Volatility Compression Breakout v1", Timeframe: "1h", WarmupBars: 354, Description: "Legacy 1h ATR compression breakout signal adapter."},
		grid:     grid,
		evaluate: func(id protocolv2.ParameterCandidateID, candles []model.Candle) ([]strategy.EntrySignal, error) {
			var values map[string]any
			for _, candidate := range grid {
				if candidate.ID == id {
					values = candidate.Values
					break
				}
			}
			if values == nil {
				return nil, fmt.Errorf("unknown vcb candidate %q", id)
			}
			report := strategy.SimulateVolatilityCompressionBreakoutV1WithParams(candles, strategy.VolatilityCompressionBreakoutV1Params{
				CompressionWindow: values["compression_window"].(int), RangeWindow: values["range_window"].(int),
				ATRCompression: values["atr_compression"].(float64), BreakoutExpansion: values["breakout_expansion"].(float64),
				TrendFilter: strategy.NR7TrendCloseEMA200, StopMode: strategy.VCBStopCompressionLow,
			})
			return report.Signals, nil
		},
	}
}

func breakoutRetest() Adapter {
	return adapter{
		metadata: execution.StrategyMetadata{Ref: ref(BreakoutRetestLongCode, "v2.0.0"), Name: "Breakout Retest Long v2", Timeframe: "15m", WarmupBars: 240, Description: "Legacy 15m breakout/retest with prior closed 1h trend signal adapter."},
		grid:     []ParameterCandidate{{ID: "default", Values: map[string]any{}}},
		evaluate: func(_ protocolv2.ParameterCandidateID, candles []model.Candle) ([]strategy.EntrySignal, error) {
			return strategy.SimulateBreakoutRetestLongV2(candles).Signals, nil
		},
	}
}

func ref(code protocolv2.StrategyCode, version protocolv2.StrategyVersion) protocolv2.StrategyRef {
	return protocolv2.StrategyRef{Code: code, Version: version}
}

func cloneValues(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneDiagnostics(values map[string]float64) map[string]float64 {
	out := make(map[string]float64, len(values)+3)
	for key, value := range values {
		if finite(value) {
			out[key] = value
		}
	}
	return out
}

func stableIDValue(id protocolv2.ParameterCandidateID) float64 {
	var value uint64
	for _, char := range id {
		value = value*33 + uint64(char)
	}
	return float64(value % 1_000_000)
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

// Metadata returns the sorted immutable core metadata without exposing mutable
// adapter implementation state.
func Metadata() []execution.StrategyMetadata {
	adapters := Core()
	metadata := make([]execution.StrategyMetadata, 0, len(adapters))
	for _, candidate := range adapters {
		metadata = append(metadata, candidate.Metadata())
	}
	sort.Slice(metadata, func(i, j int) bool { return metadata[i].Ref.String() < metadata[j].Ref.String() })
	return metadata
}

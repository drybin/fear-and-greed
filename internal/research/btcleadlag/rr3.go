package btcleadlag

import (
	"fmt"
	"sort"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/metrics"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

type RR3Report struct {
	ImpulseThreshold float64     `json:"btc_impulse_threshold"`
	Rule             string      `json:"rule"`
	Symbols          []RR3Symbol `json:"symbols"`
}

type RR3Symbol struct {
	Symbol  string          `json:"symbol"`
	Signals int             `json:"signals"`
	Metrics metrics.Summary `json:"metrics"`
}

// ShortRR3 tests a fixed BTC-down continuation rule. The stop is the high of
// the completed alt candle synchronized with BTC's impulse; entry is next open.
func ShortRR3(btc []model.Candle, alts map[string][]model.Candle, threshold float64) (RR3Report, error) {
	if threshold <= 0 || threshold >= 1 {
		return RR3Report{}, fmt.Errorf("btc lead-lag: impulse threshold must be in (0, 1)")
	}
	events := btcImpulses(btc, threshold)
	keys := make([]string, 0, len(alts))
	for symbol := range alts {
		keys = append(keys, symbol)
	}
	sort.Strings(keys)
	report := RR3Report{ImpulseThreshold: threshold, Rule: "BTC 1h close <= -threshold; short alt at next 1h open, stop at synchronized completed alt high, full take-profit at 3R, or time exit after four hours."}
	for _, symbol := range keys {
		candles := alts[symbol]
		byTime := make(map[time.Time]model.Candle, len(candles))
		engineCandles := make([]execution.Candle, 0, len(candles))
		for _, candle := range candles {
			byTime[candle.OpenTime.UTC()] = candle
			engineCandles = append(engineCandles, execution.Candle{Time: candle.OpenTime.UTC(), Open: candle.Open, High: candle.High, Low: candle.Low, Close: candle.Close, FundingRate: candle.FundingRate})
		}
		signals := make([]execution.CloseConfirmedSignal, 0)
		for _, event := range events {
			if event.direction >= 0 {
				continue
			}
			sourceTime := event.time.Add(-time.Hour)
			source, ok := byTime[sourceTime]
			entry, entryOK := byTime[event.time]
			if !ok || !entryOK || source.High <= entry.Open {
				continue
			}
			signals = append(signals, execution.CloseConfirmedSignal{
				SignalID: fmt.Sprintf("btc-down-rr3-%d", sourceTime.Unix()), Strategy: protocolv2.StrategyRef{Code: "btc-lead-lag-short-rr3-v1", Version: "v1.0.0"}, Symbol: protocolv2.Symbol(symbol), Timeframe: "1h", SourceCandleTime: sourceTime,
				Side: execution.SideShort, Stop: protocolv2.RoundPrice(source.High), TargetRiskMultiple: 3, ExitAllAtTP1: true, TimeExitAt: event.time.Add(4 * time.Hour),
			})
		}
		engine, err := execution.NewEngine(execution.Config{InitialEquity: 10000, Interval: time.Hour, CommissionBPS: 10, SlippageBPS: 5, RiskPerTradePercent: 1, MaxNotionalPercent: 20, CostProfile: "base", GapPolicy: execution.GapPolicyReject, CloseAtFoldEnd: true})
		if err != nil {
			return RR3Report{}, err
		}
		result, err := engine.Run(engineCandles, signals)
		if err != nil {
			return RR3Report{}, fmt.Errorf("run %s: %w", symbol, err)
		}
		summary, err := metrics.Calculate(metrics.Input{InitialEquity: 10000, Equity: result.Equity, Trades: result.Trades})
		if err != nil {
			return RR3Report{}, fmt.Errorf("metrics %s: %w", symbol, err)
		}
		report.Symbols = append(report.Symbols, RR3Symbol{Symbol: symbol, Signals: len(signals), Metrics: summary})
	}
	return report, nil
}

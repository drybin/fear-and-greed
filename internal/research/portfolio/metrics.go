package portfolio

import (
	"math"
	"sort"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

type Metrics struct {
	InitialCapital               float64 `json:"initial_capital"`
	FinalEquity                  float64 `json:"final_equity"`
	NetProfit                    float64 `json:"net_profit"`
	NetReturn                    float64 `json:"net_return"`
	AnnualizedReturn             float64 `json:"annualized_return"`
	MaxDrawdown                  float64 `json:"max_drawdown"`
	Calmar                       float64 `json:"calmar"`
	AverageExposure              float64 `json:"average_exposure"`
	Turnover                     float64 `json:"turnover"`
	Commission                   float64 `json:"commission"`
	Slippage                     float64 `json:"slippage"`
	TradeCount                   int     `json:"trade_count"`
	MaxConcurrentPositions       int     `json:"max_concurrent_positions"`
	AcceptedEntries              int     `json:"accepted_entries"`
	RejectedEntries              int     `json:"rejected_entries"`
	RejectedOpportunityRate      float64 `json:"rejected_opportunity_rate"`
	MaxProfitContributionPercent float64 `json:"max_profit_contribution_percent"`
}

type Benchmarks struct {
	Cash        Metrics `json:"cash"`
	BTC         Metrics `json:"btc_buy_and_hold"`
	EqualWeight Metrics `json:"equal_weight_buy_and_hold"`
}

type Decision struct {
	Status      string   `json:"status"`
	FailedGates []string `json:"failed_gates,omitempty"`
}

type Report struct {
	SchemaVersion string                  `json:"schema_version"`
	ExperimentID  protocolv2.ExperimentID `json:"experiment_id"`
	ManifestHash  protocolv2.SHA256Hex    `json:"manifest_hash"`
	GeneratedAt   time.Time               `json:"generated_at"`
	Range         protocolv2.TimeRange    `json:"range"`
	Strategy      protocolv2.StrategyRef  `json:"strategy"`
	Candidate     string                  `json:"candidate"`
	Diagnostic    bool                    `json:"diagnostic"`
	Base          Metrics                 `json:"base"`
	Stress        Metrics                 `json:"stress"`
	Benchmarks    Benchmarks              `json:"benchmarks"`
	Decision      Decision                `json:"decision"`
	Rebalances    []Rebalance             `json:"rebalances"`
	BaseResult    EngineResult            `json:"base_result"`
	StressResult  EngineResult            `json:"stress_result"`
}

func CalculateMetrics(result EngineResult) Metrics {
	metrics := Metrics{
		InitialCapital: result.InitialCapital,
		FinalEquity:    result.FinalCash,
		Commission:     result.Commission,
		Slippage:       result.Slippage,
		TradeCount:     len(result.Trades),
	}
	metrics.NetProfit = protocolv2.RoundFee(metrics.FinalEquity - metrics.InitialCapital)
	if metrics.InitialCapital > 0 {
		metrics.NetReturn = protocolv2.RoundMetric(metrics.NetProfit / metrics.InitialCapital)
		metrics.Turnover = protocolv2.RoundMetric(result.TradedNotional / metrics.InitialCapital)
	}
	if len(result.Equity) > 0 {
		days := result.Equity[len(result.Equity)-1].Time.Sub(result.Equity[0].Time).Hours()/24 + 1
		if days > 0 && metrics.FinalEquity > 0 && metrics.InitialCapital > 0 {
			metrics.AnnualizedReturn = protocolv2.RoundMetric(math.Pow(metrics.FinalEquity/metrics.InitialCapital, 365/days) - 1)
		}
		peak, exposure := result.Equity[0].Equity, 0.0
		for _, point := range result.Equity {
			if point.Equity > peak {
				peak = point.Equity
			}
			if peak > 0 {
				metrics.MaxDrawdown = math.Max(metrics.MaxDrawdown, (peak-point.Equity)/peak)
			}
			exposure += point.Exposure
			if point.OpenPositions > metrics.MaxConcurrentPositions {
				metrics.MaxConcurrentPositions = point.OpenPositions
			}
		}
		metrics.MaxDrawdown = protocolv2.RoundMetric(metrics.MaxDrawdown)
		metrics.AverageExposure = protocolv2.RoundMetric(exposure / float64(len(result.Equity)))
		if metrics.MaxDrawdown > 0 {
			metrics.Calmar = protocolv2.RoundMetric(metrics.AnnualizedReturn / metrics.MaxDrawdown)
		}
	}
	for _, allocation := range result.Decisions {
		if allocation.Action != "entry" {
			continue
		}
		if allocation.Accepted {
			metrics.AcceptedEntries++
		} else {
			metrics.RejectedEntries++
		}
	}
	opportunities := metrics.AcceptedEntries + metrics.RejectedEntries
	if opportunities > 0 {
		metrics.RejectedOpportunityRate = protocolv2.RoundMetric(float64(metrics.RejectedEntries) / float64(opportunities))
	}
	positiveTotal, largest := 0.0, 0.0
	for _, trade := range result.Trades {
		if trade.NetPnL > 0 {
			positiveTotal += trade.NetPnL
			largest = math.Max(largest, trade.NetPnL)
		}
	}
	if positiveTotal > 0 {
		metrics.MaxProfitContributionPercent = protocolv2.RoundMetric(largest / positiveTotal)
	}
	return metrics
}

func EvaluateDecision(base, stress Metrics, benchmarks Benchmarks, gates Gates, diagnostic bool) Decision {
	var failed []string
	if base.NetReturn < gates.MinNetReturn {
		failed = append(failed, "net_return")
	}
	if base.MaxDrawdown > gates.MaxDrawdown {
		failed = append(failed, "max_drawdown")
	}
	if base.NetReturn-benchmarks.BTC.NetReturn < gates.MinExcessVsBTC {
		failed = append(failed, "excess_vs_btc")
	}
	if base.NetReturn-benchmarks.EqualWeight.NetReturn < gates.MinExcessVsEqualWeight {
		failed = append(failed, "excess_vs_equal_weight")
	}
	if base.MaxProfitContributionPercent > gates.MaxContribution {
		failed = append(failed, "contribution_concentration")
	}
	if gates.RequireStressPositive && stress.NetReturn <= 0 {
		failed = append(failed, "stress_positive")
	}
	sort.Strings(failed)
	if len(failed) > 0 {
		return Decision{Status: "reject", FailedGates: failed}
	}
	if diagnostic {
		return Decision{Status: "observe"}
	}
	return Decision{Status: "portfolio-pass"}
}

func BuildBenchmarks(bars map[protocolv2.Symbol][]DailyBar, start, end time.Time, capital float64, costs CostProfile) Benchmarks {
	return Benchmarks{
		Cash:        benchmarkMetrics(capital, capital, start, end, 0, 0, 0),
		BTC:         buyAndHoldBenchmark(map[protocolv2.Symbol][]DailyBar{"BTCUSDT": bars["BTCUSDT"]}, start, end, capital, costs),
		EqualWeight: buyAndHoldBenchmark(bars, start, end, capital, costs),
	}
}

func buyAndHoldBenchmark(bars map[protocolv2.Symbol][]DailyBar, start, end time.Time, capital float64, costs CostProfile) Metrics {
	calendar := portfolioCalendar(bars, start, end)
	if len(calendar) == 0 {
		return benchmarkMetrics(capital, capital, start, end, 0, 0, 0)
	}
	index := indexBars(bars)
	firstDay, lastDay := calendar[0], calendar[len(calendar)-1]
	eligible := make([]protocolv2.Symbol, 0, len(bars))
	for symbol := range bars {
		if _, ok := index[firstDay][symbol]; ok {
			eligible = append(eligible, symbol)
		}
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i] < eligible[j] })
	if len(eligible) == 0 {
		return benchmarkMetrics(capital, capital, start, end, 0, 0, 0)
	}
	allocation := capital / float64(len(eligible))
	quantities := make(map[protocolv2.Symbol]float64, len(eligible))
	lastPrices := make(map[protocolv2.Symbol]float64, len(eligible))
	totalCommission, totalSlippage, traded := 0.0, 0.0, 0.0
	for _, symbol := range eligible {
		asset := index[firstDay][symbol]
		entry := asset.Open * (1 + costs.SlippageBPS/10000)
		entryCommission := allocation * costs.CommissionBPS / 10000
		quantities[symbol] = (allocation - entryCommission) / entry
		lastPrices[symbol] = asset.Close
		totalCommission += entryCommission
		totalSlippage += (entry - asset.Open) * quantities[symbol]
		traded += allocation
	}
	equity := []EquityPoint{{Time: firstDay, Equity: capital}}
	for _, day := range calendar {
		value := 0.0
		for _, symbol := range eligible {
			if bar, ok := index[day][symbol]; ok {
				lastPrices[symbol] = bar.Close
			}
			value += quantities[symbol] * lastPrices[symbol]
		}
		equity = append(equity, EquityPoint{Time: day, Equity: protocolv2.RoundFee(value), PositionValue: protocolv2.RoundFee(value), Exposure: 1, OpenPositions: len(eligible)})
	}
	final := 0.0
	for _, symbol := range eligible {
		reference := lastPrices[symbol]
		if bar, ok := index[lastDay][symbol]; ok {
			reference = bar.Close
		}
		exit := reference * (1 - costs.SlippageBPS/10000)
		qty := quantities[symbol]
		exitNotional := qty * exit
		exitCommission := exitNotional * costs.CommissionBPS / 10000
		final += exitNotional - exitCommission
		totalCommission += exitCommission
		totalSlippage += (reference - exit) * qty
		traded += exitNotional
	}
	result := EngineResult{InitialCapital: capital, FinalCash: protocolv2.RoundFee(final), Commission: protocolv2.RoundFee(totalCommission), Slippage: protocolv2.RoundFee(totalSlippage), TradedNotional: protocolv2.RoundFee(traded), Equity: equity}
	result.Equity[len(result.Equity)-1].Equity = result.FinalCash
	result.Equity[len(result.Equity)-1].PositionValue = 0
	result.Equity[len(result.Equity)-1].Exposure = 0
	result.Equity[len(result.Equity)-1].OpenPositions = 0
	return CalculateMetrics(result)
}

func benchmarkMetrics(initial, final float64, start, end time.Time, commission, slippage, traded float64) Metrics {
	result := EngineResult{
		InitialCapital: initial,
		FinalCash:      protocolv2.RoundFee(final),
		Commission:     protocolv2.RoundFee(commission),
		Slippage:       protocolv2.RoundFee(slippage),
		TradedNotional: protocolv2.RoundFee(traded),
		Equity: []EquityPoint{
			{Time: start, Equity: initial},
			{Time: end.Add(-24 * time.Hour), Equity: final},
		},
	}
	return CalculateMetrics(result)
}

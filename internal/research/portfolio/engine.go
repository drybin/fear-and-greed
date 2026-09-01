package portfolio

import (
	"fmt"
	"sort"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

type Position struct {
	Symbol      protocolv2.Symbol `json:"symbol"`
	OpenedAt    time.Time         `json:"opened_at"`
	Quantity    float64           `json:"quantity"`
	EntryPrice  float64           `json:"entry_price"`
	Stop        float64           `json:"stop"`
	InitialRisk float64           `json:"initial_risk"`
}

type Trade struct {
	Symbol          protocolv2.Symbol `json:"symbol"`
	OpenedAt        time.Time         `json:"opened_at"`
	ClosedAt        time.Time         `json:"closed_at"`
	EntryPrice      float64           `json:"entry_price"`
	ExitPrice       float64           `json:"exit_price"`
	Quantity        float64           `json:"quantity"`
	EntryCommission float64           `json:"entry_commission"`
	ExitCommission  float64           `json:"exit_commission"`
	NetPnL          float64           `json:"net_pnl"`
	InitialRisk     float64           `json:"initial_risk"`
	Reason          string            `json:"reason"`
}

type AllocationDecision struct {
	Time             time.Time         `json:"time"`
	Symbol           protocolv2.Symbol `json:"symbol"`
	Action           string            `json:"action"`
	Accepted         bool              `json:"accepted"`
	Reason           string            `json:"reason"`
	Rank             int               `json:"rank"`
	Score            float64           `json:"score"`
	RelativeStrength float64           `json:"relative_strength"`
	Equity           float64           `json:"equity"`
	Cash             float64           `json:"cash"`
	OpenRisk         float64           `json:"open_risk"`
}

type EquityPoint struct {
	Time          time.Time `json:"time"`
	Cash          float64   `json:"cash"`
	PositionValue float64   `json:"position_value"`
	Equity        float64   `json:"equity"`
	Exposure      float64   `json:"exposure"`
	OpenPositions int       `json:"open_positions"`
}

type EngineResult struct {
	InitialCapital float64              `json:"initial_capital"`
	FinalCash      float64              `json:"final_cash"`
	Equity         []EquityPoint        `json:"equity"`
	Trades         []Trade              `json:"trades"`
	Decisions      []AllocationDecision `json:"decisions"`
	Commission     float64              `json:"commission"`
	Slippage       float64              `json:"slippage"`
	TradedNotional float64              `json:"traded_notional"`
}

type Engine struct {
	Limits Limits
	Costs  CostProfile
}

func (e Engine) Run(bars map[protocolv2.Symbol][]DailyBar, events []Rebalance, start, end time.Time) (EngineResult, error) {
	if err := validateCosts(e.Costs); err != nil {
		return EngineResult{}, err
	}
	if !positive(e.Limits.InitialCapital) {
		return EngineResult{}, fmt.Errorf("portfolio: invalid initial capital")
	}
	calendar := portfolioCalendar(bars, start, end)
	if len(calendar) == 0 {
		return EngineResult{}, fmt.Errorf("portfolio: no evaluation bars")
	}
	index := indexBars(bars)
	eventByTime := map[time.Time]Rebalance{}
	for _, event := range events {
		eventByTime[event.FillTime] = event
	}
	result := EngineResult{InitialCapital: e.Limits.InitialCapital, FinalCash: protocolv2.RoundFee(e.Limits.InitialCapital)}
	positions := map[protocolv2.Symbol]Position{}
	lastClose := map[protocolv2.Symbol]float64{}
	for _, day := range calendar {
		daily := index[day]
		markBars := withLastPrices(daily, positions, lastClose)
		if event, ok := eventByTime[day]; ok {
			// Membership exits release cash before simultaneous entries compete.
			for _, symbol := range sortedPositions(positions) {
				if event.RegimeOn && event.Retain[symbol] {
					continue
				}
				bar, exists := daily[symbol]
				if !exists {
					continue
				}
				e.exit(&result, positions, symbol, day, bar.Open, "rebalance")
			}
			equity := e.equityAt(result.FinalCash, positions, markBars, true)
			for _, target := range sortedTargets(event.Targets) {
				if _, held := positions[target.Symbol]; held {
					continue
				}
				bar, exists := daily[target.Symbol]
				if !exists {
					result.Decisions = append(result.Decisions, decision(day, target, false, "missing_bar", equity, result.FinalCash, openRisk(positions)))
					continue
				}
				reason := e.enter(&result, positions, target, day, bar.Open, equity)
				result.Decisions = append(result.Decisions, decision(day, target, reason == "", fallback(reason, "accepted"), equity, result.FinalCash, openRisk(positions)))
			}
		}
		// Stops are evaluated conservatively after open-time allocation.
		for _, symbol := range sortedPositions(positions) {
			bar, exists := daily[symbol]
			if !exists {
				continue
			}
			position := positions[symbol]
			if bar.Open <= position.Stop {
				e.exit(&result, positions, symbol, day, bar.Open, "stop_gap")
				continue
			}
			if bar.Low <= position.Stop {
				e.exit(&result, positions, symbol, day, position.Stop, "stop")
			}
		}
		markBars = withLastPrices(daily, positions, lastClose)
		equity, value := e.mark(result.FinalCash, positions, markBars)
		exposure := 0.0
		if equity > 0 {
			exposure = value / equity
		}
		result.Equity = append(result.Equity, EquityPoint{Time: day, Cash: result.FinalCash, PositionValue: value, Equity: equity, Exposure: exposure, OpenPositions: len(positions)})
		for symbol, bar := range daily {
			lastClose[symbol] = bar.Close
		}
	}
	last := calendar[len(calendar)-1]
	for _, symbol := range sortedPositions(positions) {
		bar, ok := index[last][symbol]
		if ok {
			e.exit(&result, positions, symbol, last, bar.Close, "fold_end")
		} else if price := lastClose[symbol]; price > 0 {
			e.exit(&result, positions, symbol, last, price, "fold_end_stale")
		}
	}
	if len(result.Equity) > 0 {
		result.Equity[len(result.Equity)-1].Cash = result.FinalCash
		result.Equity[len(result.Equity)-1].PositionValue = 0
		result.Equity[len(result.Equity)-1].Equity = result.FinalCash
		result.Equity[len(result.Equity)-1].Exposure = 0
		result.Equity[len(result.Equity)-1].OpenPositions = 0
	}
	return result, nil
}

func (e Engine) enter(result *EngineResult, positions map[protocolv2.Symbol]Position, target Rank, at time.Time, open, equity float64) string {
	if len(positions) >= e.Limits.MaxPositions {
		return "position_limit"
	}
	if target.MaxEntryPrice > 0 && open > target.MaxEntryPrice {
		return "entry_extension"
	}
	entry := protocolv2.RoundPrice(open * (1 + e.Costs.SlippageBPS/10000))
	if target.StopDistance <= 0 || target.StopDistance >= entry {
		return "invalid_stop"
	}
	riskBudget := equity * e.Limits.RiskPerTradePercent / 100
	riskQty := riskBudget / target.StopDistance
	capQty := equity * e.Limits.MaxPositionPercent / 100 / entry
	qty := protocolv2.RoundQuantity(minFloat(riskQty, capQty))
	if qty <= 0 {
		return "invalid_quantity"
	}
	risk := protocolv2.RoundFee(qty * target.StopDistance)
	if openRisk(positions)+risk > equity*e.Limits.MaxAggregateRiskPct/100 {
		return "aggregate_risk_limit"
	}
	notional := protocolv2.RoundFee(entry * qty)
	commission := protocolv2.RoundFee(notional * e.Costs.CommissionBPS / 10000)
	if notional+commission > result.FinalCash {
		qty = protocolv2.RoundQuantity(result.FinalCash / (entry * (1 + e.Costs.CommissionBPS/10000)))
		notional, commission = protocolv2.RoundFee(entry*qty), protocolv2.RoundFee(entry*qty*e.Costs.CommissionBPS/10000)
		if qty <= 0 || notional+commission > result.FinalCash {
			return "insufficient_cash"
		}
		risk = protocolv2.RoundFee(qty * target.StopDistance)
	}
	result.FinalCash = protocolv2.RoundFee(result.FinalCash - notional - commission)
	result.Commission = protocolv2.RoundFee(result.Commission + commission)
	result.Slippage = protocolv2.RoundFee(result.Slippage + (entry-open)*qty)
	result.TradedNotional = protocolv2.RoundFee(result.TradedNotional + notional)
	positions[target.Symbol] = Position{Symbol: target.Symbol, OpenedAt: at, Quantity: qty, EntryPrice: entry, Stop: protocolv2.RoundPrice(entry - target.StopDistance), InitialRisk: risk}
	return ""
}

func (e Engine) exit(result *EngineResult, positions map[protocolv2.Symbol]Position, symbol protocolv2.Symbol, at time.Time, reference float64, reason string) {
	p, ok := positions[symbol]
	if !ok {
		return
	}
	exit := protocolv2.RoundPrice(reference * (1 - e.Costs.SlippageBPS/10000))
	notional := protocolv2.RoundFee(exit * p.Quantity)
	commission := protocolv2.RoundFee(notional * e.Costs.CommissionBPS / 10000)
	result.FinalCash = protocolv2.RoundFee(result.FinalCash + notional - commission)
	result.Commission = protocolv2.RoundFee(result.Commission + commission)
	result.Slippage = protocolv2.RoundFee(result.Slippage + (reference-exit)*p.Quantity)
	result.TradedNotional = protocolv2.RoundFee(result.TradedNotional + notional)
	entryCommission := protocolv2.RoundFee(p.EntryPrice * p.Quantity * e.Costs.CommissionBPS / 10000)
	pnl := protocolv2.RoundFee((exit-p.EntryPrice)*p.Quantity - entryCommission - commission)
	result.Trades = append(result.Trades, Trade{Symbol: symbol, OpenedAt: p.OpenedAt, ClosedAt: at, EntryPrice: p.EntryPrice, ExitPrice: exit, Quantity: p.Quantity, EntryCommission: entryCommission, ExitCommission: commission, NetPnL: pnl, InitialRisk: p.InitialRisk, Reason: reason})
	delete(positions, symbol)
}

func (e Engine) equityAt(cash float64, positions map[protocolv2.Symbol]Position, bars map[protocolv2.Symbol]DailyBar, useOpen bool) float64 {
	equity, _ := e.markAt(cash, positions, bars, useOpen)
	return equity
}
func (e Engine) mark(cash float64, positions map[protocolv2.Symbol]Position, bars map[protocolv2.Symbol]DailyBar) (float64, float64) {
	return e.markAt(cash, positions, bars, false)
}
func (e Engine) markAt(cash float64, positions map[protocolv2.Symbol]Position, bars map[protocolv2.Symbol]DailyBar, useOpen bool) (float64, float64) {
	value := 0.0
	for symbol, p := range positions {
		if bar, ok := bars[symbol]; ok {
			price := bar.Close
			if useOpen {
				price = bar.Open
			}
			value += price * p.Quantity
		}
	}
	return protocolv2.RoundFee(cash + value), protocolv2.RoundFee(value)
}

func indexBars(bars map[protocolv2.Symbol][]DailyBar) map[time.Time]map[protocolv2.Symbol]DailyBar {
	out := map[time.Time]map[protocolv2.Symbol]DailyBar{}
	for s, series := range bars {
		for _, b := range series {
			if out[b.Time] == nil {
				out[b.Time] = map[protocolv2.Symbol]DailyBar{}
			}
			out[b.Time][s] = b
		}
	}
	return out
}
func portfolioCalendar(bars map[protocolv2.Symbol][]DailyBar, start, end time.Time) []time.Time {
	seen := map[time.Time]bool{}
	for _, series := range bars {
		for _, b := range series {
			if !b.Time.Before(start) && b.Time.Before(end) {
				seen[b.Time] = true
			}
		}
	}
	out := make([]time.Time, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Before(out[j]) })
	return out
}
func sortedPositions(p map[protocolv2.Symbol]Position) []protocolv2.Symbol {
	out := make([]protocolv2.Symbol, 0, len(p))
	for s := range p {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
func sortedTargets(in []Rank) []Rank {
	out := append([]Rank(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rank != out[j].Rank {
			return out[i].Rank < out[j].Rank
		}
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if out[i].Return != out[j].Return {
			return out[i].Return > out[j].Return
		}
		return out[i].Symbol < out[j].Symbol
	})
	return out
}
func withLastPrices(current map[protocolv2.Symbol]DailyBar, positions map[protocolv2.Symbol]Position, last map[protocolv2.Symbol]float64) map[protocolv2.Symbol]DailyBar {
	out := make(map[protocolv2.Symbol]DailyBar, len(current)+len(positions))
	for symbol, bar := range current {
		out[symbol] = bar
	}
	for symbol := range positions {
		if _, ok := out[symbol]; ok {
			continue
		}
		if price := last[symbol]; price > 0 {
			out[symbol] = DailyBar{Open: price, High: price, Low: price, Close: price}
		}
	}
	return out
}
func openRisk(p map[protocolv2.Symbol]Position) float64 {
	total := 0.0
	for _, v := range p {
		total += v.InitialRisk
	}
	return total
}
func decision(t time.Time, rank Rank, accepted bool, reason string, equity, cash, risk float64) AllocationDecision {
	return AllocationDecision{Time: t, Symbol: rank.Symbol, Action: "entry", Accepted: accepted, Reason: reason, Rank: rank.Rank, Score: rank.Score, RelativeStrength: rank.Return, Equity: equity, Cash: cash, OpenRisk: risk}
}
func fallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

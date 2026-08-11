package metrics

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

// Input is the raw, mark-to-market execution evidence for one standalone unit.
// RiskByTrade is optional cash risk at entry, used to calculate R expectancy.
type Input struct {
	InitialEquity float64
	Equity        []execution.EquitySnapshot
	Trades        []execution.TradeState
	RiskByTrade   map[string]float64
}

// Trade is a normalized accounting view of an execution trade.
type Trade struct {
	ID, Symbol string
	OpenedAt   time.Time
	ClosedAt   *time.Time
	NetPnL     float64
	GrossPnL   float64
	RMultiple  *float64
}

// DrawdownPoint is one peak-relative mark-to-market measurement.
type DrawdownPoint struct {
	Time     time.Time `json:"time"`
	Equity   float64   `json:"equity"`
	Peak     float64   `json:"peak"`
	Percent  float64   `json:"percent"`
	Duration int64     `json:"duration_seconds"`
}

// Summary holds deterministic, non-annualized metrics unless the evidence
// covers at least one day. Ratios with no meaningful denominator are nil.
type Summary struct {
	GrossReturn                *float64            `json:"gross_return"`
	NetReturn                  *float64            `json:"net_return"`
	AnnualizedReturn           *float64            `json:"annualized_return"`
	Drawdown                   []DrawdownPoint     `json:"drawdown"`
	MaxDrawdown                float64             `json:"max_drawdown"`
	MaxDrawdownDurationSeconds int64               `json:"max_drawdown_duration_seconds"`
	Calmar                     *float64            `json:"calmar"`
	ExpectancyCurrency         *float64            `json:"expectancy_currency"`
	ExpectancyR                *float64            `json:"expectancy_r"`
	ProfitFactor               *float64            `json:"profit_factor"`
	PayoffRatio                *float64            `json:"payoff_ratio"`
	TradeCount                 int                 `json:"trade_count"`
	ClosedTradeCount           int                 `json:"closed_trade_count"`
	Wins                       int                 `json:"wins"`
	Losses                     int                 `json:"losses"`
	Breakevens                 int                 `json:"breakevens"`
	TradeWinRate               *float64            `json:"trade_win_rate"`
	AverageHoldingSeconds      *float64            `json:"average_holding_seconds"`
	MedianHoldingSeconds       *float64            `json:"median_holding_seconds"`
	Exposure                   *float64            `json:"exposure"`
	CapitalUtilization         *float64            `json:"capital_utilization"`
	Turnover                   *float64            `json:"turnover"`
	TotalCommission            float64             `json:"total_commission"`
	TotalSlippage              float64             `json:"total_slippage"`
	SymbolWinRate              map[string]*float64 `json:"symbol_win_rate"`
	Breadth                    int                 `json:"breadth"`
	ContributionConcentration  *float64            `json:"contribution_concentration"`
}

// Calculate derives metrics exclusively from recorded execution evidence.
func Calculate(in Input) (Summary, error) {
	if !finite(in.InitialEquity) || in.InitialEquity <= 0 {
		return Summary{}, fmt.Errorf("metrics: initial equity must be finite and positive")
	}
	equity := append([]execution.EquitySnapshot(nil), in.Equity...)
	sort.Slice(equity, func(i, j int) bool { return equity[i].Time.Before(equity[j].Time) })
	for i, point := range equity {
		if err := point.Validate(); err != nil {
			return Summary{}, fmt.Errorf("metrics: invalid equity snapshot: %w", err)
		}
		if i > 0 && !point.Time.After(equity[i-1].Time) {
			return Summary{}, fmt.Errorf("metrics: equity snapshots must be strictly chronological")
		}
	}
	out := Summary{SymbolWinRate: map[string]*float64{}}
	trades := make([]Trade, 0, len(in.Trades))
	for _, state := range in.Trades {
		trade, err := normalizeTrade(state, in.RiskByTrade[state.TradeID])
		if err != nil {
			return Summary{}, err
		}
		trades = append(trades, trade)
	}
	out.TradeCount = len(trades)
	out.ClosedTradeCount, out.ExpectancyCurrency, out.ExpectancyR = expectancy(trades)
	classify(&out, trades)
	holding(&out, trades)
	costsAndTurnover(&out, trades, in.Trades, equity)
	symbolMetrics(&out, trades)
	if len(equity) == 0 {
		return out, nil
	}
	final := equity[len(equity)-1]
	out.TotalCommission, out.TotalSlippage = final.CommissionCosts, final.SlippageCosts
	net := protocolv2.RoundMetric(final.TotalEquity/in.InitialEquity - 1)
	gross := protocolv2.RoundMetric((final.TotalEquity+out.TotalCommission+out.TotalSlippage)/in.InitialEquity - 1)
	out.NetReturn, out.GrossReturn = ptr(net), ptr(gross)
	out.Drawdown, out.MaxDrawdown, out.MaxDrawdownDurationSeconds = drawdowns(equity)
	if duration := final.Time.Sub(equity[0].Time); duration >= 24*time.Hour && final.TotalEquity > 0 {
		annual := protocolv2.RoundMetric(math.Pow(final.TotalEquity/in.InitialEquity, 365.25/(duration.Hours()/24)) - 1)
		out.AnnualizedReturn = ptr(annual)
		if out.MaxDrawdown > 0 {
			out.Calmar = ptr(protocolv2.RoundMetric(annual / out.MaxDrawdown))
		}
	}
	exposure(&out, equity)
	return out, nil
}

func normalizeTrade(t execution.TradeState, risk float64) (Trade, error) {
	if err := t.Validate(); err != nil {
		return Trade{}, fmt.Errorf("metrics: invalid trade %s: %w", t.TradeID, err)
	}
	result := Trade{ID: t.TradeID, Symbol: string(t.Entry.Symbol), OpenedAt: t.Entry.FillTime}
	entry := t.Entry.Price*t.Entry.Quantity + t.Entry.Commission
	proceeds, commissions := 0.0, t.Entry.Commission
	exits := append([]execution.PartialExitFill(nil), t.PartialExits...)
	if t.FinalExit != nil {
		exits = append(exits, execution.PartialExitFill{FillAudit: t.FinalExit.FillAudit, PositionID: t.FinalExit.PositionID, Reason: t.FinalExit.Reason})
	}
	for _, fill := range exits {
		proceeds += fill.Price * fill.Quantity
		commissions += fill.Commission
	}
	result.GrossPnL = protocolv2.RoundFee(proceeds - t.Entry.Price*t.Entry.Quantity)
	result.NetPnL = protocolv2.RoundFee(proceeds - entry - (commissions - t.Entry.Commission))
	if t.FinalExit != nil {
		closed := t.FinalExit.FillTime
		result.ClosedAt = &closed
	}
	if risk > 0 && finite(risk) {
		r := protocolv2.RoundMetric(result.NetPnL / risk)
		result.RMultiple = &r
	}
	return result, nil
}

func expectancy(trades []Trade) (int, *float64, *float64) {
	var total, totalR float64
	closed, rCount := 0, 0
	for _, t := range trades {
		if t.ClosedAt == nil {
			continue
		}
		closed++
		total += t.NetPnL
		if t.RMultiple != nil {
			totalR += *t.RMultiple
			rCount++
		}
	}
	if closed == 0 {
		return 0, nil, nil
	}
	currency := ptr(protocolv2.RoundMetric(total / float64(closed)))
	if rCount == 0 {
		return closed, currency, nil
	}
	return closed, currency, ptr(protocolv2.RoundMetric(totalR / float64(rCount)))
}

func classify(out *Summary, trades []Trade) {
	var winSum, lossSum float64
	for _, t := range trades {
		if t.ClosedAt == nil {
			continue
		}
		switch {
		case t.NetPnL > 0:
			out.Wins++
			winSum += t.NetPnL
		case t.NetPnL < 0:
			out.Losses++
			lossSum += -t.NetPnL
		default:
			out.Breakevens++
		}
	}
	closed := out.Wins + out.Losses + out.Breakevens
	if closed > 0 {
		out.TradeWinRate = ptr(protocolv2.RoundMetric(float64(out.Wins) / float64(closed)))
	}
	if out.Losses > 0 {
		out.ProfitFactor = ptr(protocolv2.RoundMetric(winSum / lossSum))
	}
	if out.Wins > 0 && out.Losses > 0 {
		out.PayoffRatio = ptr(protocolv2.RoundMetric((winSum / float64(out.Wins)) / (lossSum / float64(out.Losses))))
	}
}

func holding(out *Summary, trades []Trade) {
	var values []float64
	for _, t := range trades {
		if t.ClosedAt != nil {
			values = append(values, t.ClosedAt.Sub(t.OpenedAt).Seconds())
		}
	}
	if len(values) == 0 {
		return
	}
	sort.Float64s(values)
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	median := values[len(values)/2]
	if len(values)%2 == 0 {
		median = (values[len(values)/2-1] + median) / 2
	}
	out.AverageHoldingSeconds, out.MedianHoldingSeconds = ptr(protocolv2.RoundMetric(sum/float64(len(values)))), ptr(protocolv2.RoundMetric(median))
}

func costsAndTurnover(out *Summary, trades []Trade, raw []execution.TradeState, equity []execution.EquitySnapshot) {
	notional := 0.0
	for _, t := range raw {
		notional += t.Entry.Price * t.Entry.Quantity
		for _, p := range t.PartialExits {
			notional += p.Price * p.Quantity
		}
		if t.FinalExit != nil {
			notional += t.FinalExit.Price * t.FinalExit.Quantity
		}
	}
	if len(equity) > 0 {
		total := 0.0
		for _, e := range equity {
			total += e.TotalEquity
		}
		if total > 0 {
			out.Turnover = ptr(protocolv2.RoundMetric(notional / (total / float64(len(equity)))))
		}
	}
}

func drawdowns(equity []execution.EquitySnapshot) ([]DrawdownPoint, float64, int64) {
	points := make([]DrawdownPoint, 0, len(equity))
	peak, maxDD := 0.0, 0.0
	var peakAt time.Time
	var maxDuration int64
	for _, e := range equity {
		if e.TotalEquity >= peak {
			peak, peakAt = e.TotalEquity, e.Time
		}
		dd := 0.0
		if peak > 0 {
			dd = protocolv2.RoundMetric((peak - e.TotalEquity) / peak)
		}
		duration := e.Time.Sub(peakAt).Seconds()
		points = append(points, DrawdownPoint{Time: e.Time, Equity: e.TotalEquity, Peak: peak, Percent: dd, Duration: int64(duration)})
		if dd > maxDD {
			maxDD = dd
		}
		if dd > 0 && int64(duration) > maxDuration {
			maxDuration = int64(duration)
		}
	}
	return points, protocolv2.RoundMetric(maxDD), maxDuration
}

func exposure(out *Summary, equity []execution.EquitySnapshot) {
	if len(equity) == 0 {
		return
	}
	if len(equity) == 1 {
		exposed := 0.0
		if equity[0].OpenPositionValue > 0 {
			exposed = 1
		}
		out.Exposure = ptr(exposed)
		if equity[0].TotalEquity > 0 {
			out.CapitalUtilization = ptr(protocolv2.RoundMetric(equity[0].OpenPositionValue / equity[0].TotalEquity))
		}
		return
	}
	totalSeconds, exposedSeconds, utilizationSeconds := 0.0, 0.0, 0.0
	for i := 0; i < len(equity)-1; i++ {
		seconds := equity[i+1].Time.Sub(equity[i].Time).Seconds()
		totalSeconds += seconds
		if equity[i].OpenPositionValue > 0 {
			exposedSeconds += seconds
		}
		if equity[i].TotalEquity > 0 {
			utilizationSeconds += seconds * equity[i].OpenPositionValue / equity[i].TotalEquity
		}
	}
	if totalSeconds > 0 {
		out.Exposure = ptr(protocolv2.RoundMetric(exposedSeconds / totalSeconds))
		out.CapitalUtilization = ptr(protocolv2.RoundMetric(utilizationSeconds / totalSeconds))
	}
}

func symbolMetrics(out *Summary, trades []Trade) {
	type counts struct {
		wins, closed int
		pnl          float64
	}
	perSymbol := map[string]*counts{}
	for _, t := range trades {
		if t.ClosedAt == nil {
			continue
		}
		c := perSymbol[t.Symbol]
		if c == nil {
			c = &counts{}
			perSymbol[t.Symbol] = c
		}
		c.closed++
		if t.NetPnL > 0 {
			c.wins++
		}
		c.pnl += t.NetPnL
	}
	var squares, totalAbs float64
	for symbol, c := range perSymbol {
		out.SymbolWinRate[symbol] = ptr(protocolv2.RoundMetric(float64(c.wins) / float64(c.closed)))
		out.Breadth++
		totalAbs += math.Abs(c.pnl)
	}
	if totalAbs > 0 {
		for _, c := range perSymbol {
			share := math.Abs(c.pnl) / totalAbs
			squares += share * share
		}
		out.ContributionConcentration = ptr(protocolv2.RoundMetric(squares))
	}
}

// FoldConsistency is the share of completed folds with positive net return.
func FoldConsistency(folds []Summary) *float64 {
	if len(folds) == 0 {
		return nil
	}
	positive := 0
	for _, fold := range folds {
		if fold.NetReturn != nil && *fold.NetReturn > 0 {
			positive++
		}
	}
	return ptr(protocolv2.RoundMetric(float64(positive) / float64(len(folds))))
}

func ptr(v float64) *float64 { return &v }
func finite(v float64) bool  { return !math.IsNaN(v) && !math.IsInf(v, 0) }

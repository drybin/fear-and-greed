package execution

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

// GapPolicy controls what happens when the interval immediately after a signal
// is absent. FillNextAvailable is the conservative protocol-v2 default: it
// never invents a price and fills at the next observed open.
type GapPolicy string

const (
	GapPolicyReject            GapPolicy = "reject-signal"
	GapPolicyFillNextAvailable GapPolicy = "fill-next-available"
)

// Candle is one already-validated OHLC evaluation interval. Time is its open.
type Candle struct {
	Time       time.Time
	Open, High float64
	Low, Close float64
}

// Config freezes the execution inputs for one (strategy, symbol, fold)
// standalone account. Zero-valued GapPolicy defaults to FillNextAvailable.
type Config struct {
	InitialEquity       float64
	Interval            time.Duration
	CommissionBPS       float64
	SlippageBPS         float64
	RiskPerTradePercent float64
	MaxNotionalPercent  float64
	CostProfile         protocolv2.CostProfileID
	GapPolicy           GapPolicy
	TimeExitBars        int
	CloseAtFoldEnd      bool
}

func (c Config) normalized() (Config, error) {
	if c.GapPolicy == "" {
		c.GapPolicy = GapPolicyFillNextAvailable
	}
	if c.RiskPerTradePercent == 0 {
		c.RiskPerTradePercent = 1
	}
	if c.MaxNotionalPercent == 0 {
		c.MaxNotionalPercent = 20
	}
	if c.InitialEquity <= 0 || !finite(c.InitialEquity) || c.Interval <= 0 ||
		c.CommissionBPS < 0 || !finite(c.CommissionBPS) ||
		c.SlippageBPS < 0 || !finite(c.SlippageBPS) ||
		c.RiskPerTradePercent <= 0 || c.RiskPerTradePercent > 100 || !finite(c.RiskPerTradePercent) ||
		c.MaxNotionalPercent <= 0 || c.MaxNotionalPercent > 100 || !finite(c.MaxNotionalPercent) ||
		c.TimeExitBars < 0 {
		return c, fmt.Errorf("execution: invalid engine config")
	}
	if c.GapPolicy != GapPolicyReject && c.GapPolicy != GapPolicyFillNextAvailable {
		return c, fmt.Errorf("execution: invalid gap policy %q", c.GapPolicy)
	}
	if err := c.CostProfile.Validate(); err != nil {
		return c, err
	}
	return c, nil
}

// AuditEvent records each engine decision, including non-fill actions such as
// delayed fills, stop-first resolution, and fold-end marking.
type AuditEvent struct {
	Time     time.Time
	Kind     string
	SignalID string
	Details  map[string]float64
}

// Result retains raw execution evidence. Selection code must use Equity, never
// RealizedCash, because the latter deliberately excludes open positions.
type Result struct {
	Trades       []TradeState
	Positions    []PositionState
	Rejections   []SignalRejection
	Equity       []EquitySnapshot
	Audit        []AuditEvent
	RealizedCash float64
}

// Engine owns one isolated long-only account. It is intentionally not a shared
// capital portfolio simulator.
type Engine struct{ config Config }

func NewEngine(config Config) (*Engine, error) {
	config, err := config.normalized()
	if err != nil {
		return nil, err
	}
	return &Engine{config: config}, nil
}

// Run processes candles chronologically. Signals are submitted independently
// but may only execute after their source candle time.
func (e *Engine) Run(candles []Candle, signals []CloseConfirmedSignal) (Result, error) {
	return e.RunWithExits(candles, signals, nil)
}

// RunWithExits processes causal entry and discretionary exit signals. Exit
// signals are evaluated before entries at the same candle open, so a closing
// long never overlaps a new long in this standalone account.
func (e *Engine) RunWithExits(candles []Candle, signals []CloseConfirmedSignal, exits []CloseConfirmedExitSignal) (Result, error) {
	if e == nil {
		return Result{}, fmt.Errorf("execution: nil engine")
	}
	if err := validateCandles(candles); err != nil {
		return Result{}, err
	}
	for _, signal := range signals {
		if err := signal.Validate(); err != nil {
			return Result{}, err
		}
	}
	for _, signal := range exits {
		if err := signal.Validate(); err != nil {
			return Result{}, err
		}
	}
	sort.Slice(signals, func(i, j int) bool {
		if signals[i].SourceCandleTime.Equal(signals[j].SourceCandleTime) {
			return signals[i].SignalID < signals[j].SignalID
		}
		return signals[i].SourceCandleTime.Before(signals[j].SourceCandleTime)
	})
	sort.Slice(exits, func(i, j int) bool {
		if exits[i].SourceCandleTime.Equal(exits[j].SourceCandleTime) {
			return exits[i].SignalID < exits[j].SignalID
		}
		return exits[i].SourceCandleTime.Before(exits[j].SourceCandleTime)
	})

	a := account{engine: e, cash: protocolv2.RoundFee(e.config.InitialEquity), initial: protocolv2.RoundFee(e.config.InitialEquity)}
	pending := append([]CloseConfirmedSignal(nil), signals...)
	pendingExits := append([]CloseConfirmedExitSignal(nil), exits...)
	for barIndex, candle := range candles {
		pendingExits = a.exitSignalsAt(candle, pendingExits)
		pending = a.entriesAt(candle, barIndex, pending)
		a.exitsAt(candle, barIndex)
		if barIndex == len(candles)-1 && e.config.CloseAtFoldEnd {
			a.closeAtFoldEnd(candle)
		}
		a.snapshot(candle.Time, candle.Close)
	}
	if len(candles) > 0 && !e.config.CloseAtFoldEnd && a.position != nil {
		a.audit = append(a.audit, AuditEvent{Time: candles[len(candles)-1].Time, Kind: "fold_end_mark", SignalID: a.position.signal.SignalID})
	}
	for _, signal := range pending {
		a.reject(signal, candles[len(candles)-1].Time, protocolv2.RejectionMissingNextBar, nil)
	}
	return a.result(), nil
}

type openPosition struct {
	state       PositionState
	trade       TradeState
	signal      CloseConfirmedSignal
	entryBar    int
	tp1Complete bool
}

type account struct {
	engine                               *Engine
	initial, cash, realized, commissions float64
	slippage                             float64
	position                             *openPosition
	trades                               []TradeState
	rejections                           []SignalRejection
	equity                               []EquitySnapshot
	audit                                []AuditEvent
	peak                                 float64
}

func (a *account) exitSignalsAt(c Candle, pending []CloseConfirmedExitSignal) []CloseConfirmedExitSignal {
	out := pending[:0]
	for _, s := range pending {
		expected := s.SourceCandleTime.Add(a.engine.config.Interval)
		if c.Time.Before(expected) {
			out = append(out, s)
			continue
		}
		if c.Time.After(expected) && a.engine.config.GapPolicy == GapPolicyReject {
			a.audit = append(a.audit, AuditEvent{Time: c.Time, Kind: "exit_signal_gap_rejected", SignalID: s.SignalID, Details: map[string]float64{"expected_time_unix": float64(expected.Unix())}})
			continue
		}
		if a.position == nil || a.position.state.Strategy != s.Strategy || a.position.state.Symbol != s.Symbol {
			a.audit = append(a.audit, AuditEvent{Time: c.Time, Kind: "exit_signal_ignored", SignalID: s.SignalID, Details: s.Diagnostics})
			continue
		}
		a.exit(c, c.Open, a.position.state.RemainingQuantity, ExitReasonSignal)
		a.audit = append(a.audit, AuditEvent{Time: c.Time, Kind: "exit_signal_fill", SignalID: s.SignalID, Details: s.Diagnostics})
	}
	return out
}

func (a *account) entriesAt(c Candle, barIndex int, pending []CloseConfirmedSignal) []CloseConfirmedSignal {
	out := pending[:0]
	for _, s := range pending {
		expected := s.SourceCandleTime.Add(a.engine.config.Interval)
		if c.Time.Before(expected) {
			out = append(out, s) // The signal is unknown until its aggregate candle closes.
			continue
		}
		if c.Time.After(expected) && a.engine.config.GapPolicy == GapPolicyReject {
			a.reject(s, c.Time, protocolv2.RejectionMissingNextBar, map[string]float64{"expected_time_unix": float64(expected.Unix())})
			continue
		}
		if a.position != nil {
			out = append(out, s) // no concurrent positions in a standalone account
			continue
		}
		a.enter(s, c, barIndex)
	}
	return out
}

func (a *account) enter(s CloseConfirmedSignal, c Candle, barIndex int) {
	entry := a.buyPrice(c.Open)
	distance := entry - s.Stop
	if !finite(distance) || distance <= 0 {
		a.reject(s, c.Time, protocolv2.RejectionInvalidStop, map[string]float64{"entry_price": entry, "stop": s.Stop})
		return
	}
	equity := a.equityAt(c.Close)
	riskQty := equity * a.engine.config.RiskPerTradePercent / 100 / distance
	capQty := equity * a.engine.config.MaxNotionalPercent / 100 / entry
	qty := protocolv2.RoundQuantity(math.Min(riskQty, capQty))
	if qty <= 0 || !finite(qty) {
		a.reject(s, c.Time, protocolv2.RejectionInvalidQuantity, map[string]float64{"risk_quantity": riskQty, "cap_quantity": capQty})
		return
	}
	notional := protocolv2.RoundFee(entry * qty)
	commission := a.commission(notional)
	if a.cash < protocolv2.RoundFee(notional+commission) {
		a.reject(s, c.Time, protocolv2.RejectionInsufficientCash, map[string]float64{"cash": a.cash, "required_cash": protocolv2.RoundFee(notional + commission)})
		return
	}
	a.cash = protocolv2.RoundFee(a.cash - notional - commission)
	a.commissions = protocolv2.RoundFee(a.commissions + commission)
	a.slippage = protocolv2.RoundFee(a.slippage + protocolv2.RoundFee((entry-c.Open)*qty))
	intent := OrderIntent{IntentID: "intent-" + s.SignalID, SignalID: s.SignalID, Strategy: s.Strategy, Symbol: s.Symbol, Side: s.Side, SourceCandleTime: s.SourceCandleTime, EligibleAt: c.Time, Quantity: qty, Stop: s.Stop, Targets: s.Targets}
	fill := EntryFill{PositionID: "position-" + s.SignalID, FillAudit: a.fillAudit("entry-"+s.SignalID, intent, c.Time, c.Open, entry, qty, commission)}
	state := PositionState{PositionID: fill.PositionID, Strategy: s.Strategy, Symbol: s.Symbol, Side: SideLong, Status: PositionOpen, OpenedAt: c.Time, InitialQuantity: qty, RemainingQuantity: qty, AverageEntryPrice: entry, Stop: s.Stop}
	a.position = &openPosition{state: state, signal: s, entryBar: barIndex, trade: TradeState{TradeID: "trade-" + s.SignalID, PositionID: fill.PositionID, Status: TradeOpen, Entry: fill}}
	a.audit = append(a.audit, AuditEvent{Time: c.Time, Kind: "entry_fill", SignalID: s.SignalID, Details: map[string]float64{"reference_price": c.Open, "price": entry, "quantity": qty, "commission": commission}})
}

func (a *account) exitsAt(c Candle, barIndex int) {
	p := a.position
	if p == nil {
		return
	}
	if c.Open <= p.state.Stop {
		a.exit(c, c.Open, p.state.RemainingQuantity, p.stopReason())
		return
	}
	if c.Low <= p.state.Stop { // stop first when high also reaches a target.
		a.exit(c, p.state.Stop, p.state.RemainingQuantity, p.stopReason())
		return
	}
	targets := sortedTargets(p.signal.Targets)
	if len(targets) > 0 && !p.tp1Complete && c.High >= targets[0].Price {
		qty := protocolv2.RoundQuantity(p.state.InitialQuantity / 2)
		if qty <= 0 || qty >= p.state.RemainingQuantity {
			a.exit(c, targets[0].Price, p.state.RemainingQuantity, ExitReasonTarget)
			return
		}
		a.exit(c, targets[0].Price, qty, ExitReasonTarget)
		if a.position != nil {
			a.position.tp1Complete = true
			a.position.state.Stop = a.position.state.AverageEntryPrice
			a.audit = append(a.audit, AuditEvent{Time: c.Time, Kind: "tp1_breakeven", SignalID: p.signal.SignalID})
		}
		return
	}
	if p.tp1Complete && len(targets) > 1 && c.High >= targets[1].Price {
		a.exit(c, targets[1].Price, p.state.RemainingQuantity, ExitReasonTarget)
		return
	}
	if a.engine.config.TimeExitBars > 0 && barIndex-p.entryBar >= a.engine.config.TimeExitBars {
		a.exit(c, c.Close, p.state.RemainingQuantity, ExitReasonTime)
	}
}

func (p *openPosition) stopReason() ExitReason {
	if p.tp1Complete && p.state.Stop == p.state.AverageEntryPrice {
		return ExitReasonBreakeven
	}
	return ExitReasonStop
}

func (a *account) exit(c Candle, reference, qty float64, reason ExitReason) {
	p := a.position
	if p == nil || qty <= 0 || qty > p.state.RemainingQuantity {
		return
	}
	price := a.sellPrice(reference)
	notional := protocolv2.RoundFee(price * qty)
	commission := a.commission(notional)
	a.cash = protocolv2.RoundFee(a.cash + notional - commission)
	a.commissions = protocolv2.RoundFee(a.commissions + commission)
	a.slippage = protocolv2.RoundFee(a.slippage + protocolv2.RoundFee((reference-price)*qty))
	a.realized = protocolv2.RoundFee(a.realized + protocolv2.RoundFee((price-p.state.AverageEntryPrice)*qty) - commission)
	intent := OrderIntent{IntentID: "intent-" + p.signal.SignalID, SignalID: p.signal.SignalID, Strategy: p.signal.Strategy, Symbol: p.signal.Symbol, Side: SideLong, SourceCandleTime: p.signal.SourceCandleTime, EligibleAt: c.Time, Quantity: qty, Stop: p.state.Stop}
	fill := a.fillAudit("exit-"+p.signal.SignalID+"-"+fmt.Sprint(len(p.trade.PartialExits)+1), intent, c.Time, reference, price, qty, commission)
	p.state.RemainingQuantity = protocolv2.RoundQuantity(p.state.RemainingQuantity - qty)
	if p.state.RemainingQuantity == 0 {
		p.state.Status = PositionClosed
		final := FinalExitFill{PositionID: p.state.PositionID, Reason: reason, FillAudit: fill}
		p.trade.Status, p.trade.FinalExit = TradeClosed, &final
		a.trades = append(a.trades, p.trade)
		a.audit = append(a.audit, AuditEvent{Time: c.Time, Kind: "final_exit", SignalID: p.signal.SignalID, Details: map[string]float64{"price": price, "quantity": qty, "commission": commission}})
		a.position = nil
		return
	}
	p.trade.PartialExits = append(p.trade.PartialExits, PartialExitFill{PositionID: p.state.PositionID, Reason: reason, FillAudit: fill})
	a.audit = append(a.audit, AuditEvent{Time: c.Time, Kind: "partial_exit", SignalID: p.signal.SignalID, Details: map[string]float64{"price": price, "quantity": qty, "commission": commission}})
}

func (a *account) closeAtFoldEnd(c Candle) {
	if a.position != nil {
		a.exit(c, c.Close, a.position.state.RemainingQuantity, ExitReasonFoldEnd)
	}
}

func (a *account) snapshot(t time.Time, mark float64) {
	open := 0.0
	unrealized := 0.0
	if a.position != nil {
		price := a.sellPrice(mark)
		open = protocolv2.RoundFee(price * a.position.state.RemainingQuantity)
		unrealized = protocolv2.RoundFee((price - a.position.state.AverageEntryPrice) * a.position.state.RemainingQuantity)
	}
	total := protocolv2.RoundFee(a.cash + open)
	if total > a.peak {
		a.peak = total
	}
	underwater := 0.0
	if a.peak > 0 {
		underwater = protocolv2.RoundMetric((total/a.peak - 1) * 100)
	}
	a.equity = append(a.equity, EquitySnapshot{Time: t, Cash: a.cash, OpenPositionValue: open, RealizedPnL: a.realized, UnrealizedPnL: unrealized, CommissionCosts: a.commissions, SlippageCosts: a.slippage, TotalEquity: total, UnderwaterPercent: underwater})
}

func (a *account) equityAt(mark float64) float64 {
	if a.position == nil {
		return a.cash
	}
	return protocolv2.RoundFee(a.cash + a.sellPrice(mark)*a.position.state.RemainingQuantity)
}

func (a *account) fillAudit(id string, intent OrderIntent, t time.Time, reference, price, qty, commission float64) FillAudit {
	slippage := protocolv2.RoundFee(math.Abs(price-reference) * qty)
	return FillAudit{FillID: id, IntentID: intent.IntentID, SignalID: intent.SignalID, Strategy: intent.Strategy, Symbol: intent.Symbol, Side: SideLong, SourceCandleTime: intent.SourceCandleTime, FillTime: t, ReferencePrice: protocolv2.RoundPrice(reference), Price: protocolv2.RoundPrice(price), Quantity: qty, Commission: commission, Slippage: slippage, CostProfile: a.engine.config.CostProfile, Audit: map[string]float64{"commission_bps": a.engine.config.CommissionBPS, "slippage_bps": a.engine.config.SlippageBPS}}
}

func (a *account) reject(s CloseConfirmedSignal, t time.Time, reason protocolv2.RejectionReason, diagnostics map[string]float64) {
	a.rejections = append(a.rejections, SignalRejection{SignalID: s.SignalID, Strategy: s.Strategy, Symbol: s.Symbol, OccurredAt: t, Reason: reason, Diagnostics: diagnostics})
	a.audit = append(a.audit, AuditEvent{Time: t, Kind: "rejection", SignalID: s.SignalID, Details: diagnostics})
}

func (a *account) result() Result {
	result := Result{Trades: append([]TradeState(nil), a.trades...), Rejections: append([]SignalRejection(nil), a.rejections...), Equity: append([]EquitySnapshot(nil), a.equity...), Audit: append([]AuditEvent(nil), a.audit...), RealizedCash: a.cash}
	if a.position != nil {
		result.Positions = []PositionState{a.position.state}
		result.Trades = append(result.Trades, a.position.trade)
	}
	return result
}

func (a *account) buyPrice(reference float64) float64 {
	return protocolv2.RoundPrice(reference * (1 + a.engine.config.SlippageBPS/10000))
}
func (a *account) sellPrice(reference float64) float64 {
	return protocolv2.RoundPrice(reference * (1 - a.engine.config.SlippageBPS/10000))
}
func (a *account) commission(notional float64) float64 {
	return protocolv2.RoundFee(notional * a.engine.config.CommissionBPS / 10000)
}

func validateCandles(candles []Candle) error {
	if len(candles) == 0 {
		return fmt.Errorf("execution: candles are required")
	}
	for i, c := range candles {
		if err := validateUTC("candle time", c.Time); err != nil {
			return err
		}
		for name, value := range map[string]float64{"open": c.Open, "high": c.High, "low": c.Low, "close": c.Close} {
			if err := validatePrice("candle "+name, protocolv2.RoundPrice(value)); err != nil || value != protocolv2.RoundPrice(value) {
				return fmt.Errorf("execution: invalid candle %s", name)
			}
		}
		if c.Low > math.Min(c.Open, c.Close) || c.High < math.Max(c.Open, c.Close) || c.Low > c.High {
			return fmt.Errorf("execution: invalid candle range")
		}
		if i > 0 && !c.Time.After(candles[i-1].Time) {
			return fmt.Errorf("execution: candles must be strictly chronological")
		}
	}
	return nil
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

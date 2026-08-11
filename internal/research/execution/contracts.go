package execution

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

// Side is the direction of a spot order. Protocol-v2 core supports long-only
// execution, but keeps the direction explicit in every audit record.
type Side string

const (
	SideLong Side = "long"
)

func (s Side) Validate() error {
	if s != SideLong {
		return fmt.Errorf("execution: unsupported side %q", s)
	}
	return nil
}

// StrategyMetadata is the immutable, registry-visible description of a
// protocol-v2 strategy adapter.
type StrategyMetadata struct {
	Ref         protocolv2.StrategyRef `json:"ref"`
	Name        string                 `json:"name"`
	Timeframe   protocolv2.Timeframe   `json:"timeframe"`
	WarmupBars  int                    `json:"warmup_bars"`
	Description string                 `json:"description,omitempty"`
}

func (m StrategyMetadata) Validate() error {
	if err := m.Ref.Validate(); err != nil {
		return err
	}
	if m.Name == "" {
		return fmt.Errorf("execution: strategy name is required")
	}
	if err := m.Timeframe.Validate(); err != nil {
		return err
	}
	if m.WarmupBars < 0 {
		return fmt.Errorf("execution: warmup bars must not be negative")
	}
	return nil
}

// Strategy supplies registry metadata. Signal evaluation is deliberately not
// part of this contract until candidate adapters are introduced.
type Strategy interface {
	Metadata() StrategyMetadata
}

// Target is an optional strategy-proposed exit level.
type Target struct {
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

func (t Target) Validate() error {
	if t.Name == "" {
		return fmt.Errorf("execution: target name is required")
	}
	return validatePrice("target price", t.Price)
}

// CloseConfirmedSignal identifies an aggregate candle by its opening timestamp.
// The decision becomes available at SourceCandleTime + strategy timeframe and
// can only fill from that timestamp onward.
type CloseConfirmedSignal struct {
	SignalID         string                 `json:"signal_id"`
	Strategy         protocolv2.StrategyRef `json:"strategy"`
	Symbol           protocolv2.Symbol      `json:"symbol"`
	Timeframe        protocolv2.Timeframe   `json:"timeframe"`
	SourceCandleTime time.Time              `json:"source_candle_time"`
	Side             Side                   `json:"side"`
	Stop             float64                `json:"stop"`
	Targets          []Target               `json:"targets,omitempty"`
	Diagnostics      map[string]float64     `json:"diagnostics,omitempty"`
}

func (s CloseConfirmedSignal) Validate() error {
	if s.SignalID == "" {
		return fmt.Errorf("execution: signal id is required")
	}
	if err := s.Strategy.Validate(); err != nil {
		return err
	}
	if err := s.Symbol.Validate(); err != nil {
		return err
	}
	if err := s.Timeframe.Validate(); err != nil {
		return err
	}
	if err := validateUTC("source candle time", s.SourceCandleTime); err != nil {
		return err
	}
	if err := s.Side.Validate(); err != nil {
		return err
	}
	if err := validatePrice("stop", s.Stop); err != nil {
		return err
	}
	seenTargets := make(map[string]struct{}, len(s.Targets))
	for _, target := range s.Targets {
		if err := target.Validate(); err != nil {
			return err
		}
		if _, duplicate := seenTargets[target.Name]; duplicate {
			return fmt.Errorf("execution: duplicate target %q", target.Name)
		}
		seenTargets[target.Name] = struct{}{}
	}
	return validateDiagnostics(s.Diagnostics)
}

// CloseConfirmedExitSignal closes an existing long position no earlier than the
// next available candle open. It lets long/cash controls express a causal
// regime change without treating the exit as a stop-loss.
type CloseConfirmedExitSignal struct {
	SignalID         string                 `json:"signal_id"`
	Strategy         protocolv2.StrategyRef `json:"strategy"`
	Symbol           protocolv2.Symbol      `json:"symbol"`
	SourceCandleTime time.Time              `json:"source_candle_time"`
	Diagnostics      map[string]float64     `json:"diagnostics,omitempty"`
}

func (s CloseConfirmedExitSignal) Validate() error {
	if s.SignalID == "" {
		return fmt.Errorf("execution: exit signal id is required")
	}
	if err := s.Strategy.Validate(); err != nil {
		return err
	}
	if err := s.Symbol.Validate(); err != nil {
		return err
	}
	if err := validateUTC("exit source candle time", s.SourceCandleTime); err != nil {
		return err
	}
	return validateDiagnostics(s.Diagnostics)
}

// OrderIntent converts an accepted signal into a quantity-bearing future order.
// EligibleAt is intentionally separate from SourceCandleTime to preserve
// next-bar causal execution.
type OrderIntent struct {
	IntentID         string                 `json:"intent_id"`
	SignalID         string                 `json:"signal_id"`
	Strategy         protocolv2.StrategyRef `json:"strategy"`
	Symbol           protocolv2.Symbol      `json:"symbol"`
	Side             Side                   `json:"side"`
	SourceCandleTime time.Time              `json:"source_candle_time"`
	EligibleAt       time.Time              `json:"eligible_at"`
	Quantity         float64                `json:"quantity"`
	Stop             float64                `json:"stop"`
	Targets          []Target               `json:"targets,omitempty"`
}

func (i OrderIntent) Validate() error {
	if i.IntentID == "" || i.SignalID == "" {
		return fmt.Errorf("execution: intent and signal ids are required")
	}
	if err := i.Strategy.Validate(); err != nil {
		return err
	}
	if err := i.Symbol.Validate(); err != nil {
		return err
	}
	if err := i.Side.Validate(); err != nil {
		return err
	}
	if err := validateUTC("source candle time", i.SourceCandleTime); err != nil {
		return err
	}
	if err := validateUTC("eligible at", i.EligibleAt); err != nil {
		return err
	}
	if !i.EligibleAt.After(i.SourceCandleTime) {
		return fmt.Errorf("execution: eligible time must be after source candle time")
	}
	if err := validateQuantity("intent quantity", i.Quantity); err != nil {
		return err
	}
	if err := validatePrice("stop", i.Stop); err != nil {
		return err
	}
	for _, target := range i.Targets {
		if err := target.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// ExitReason captures why an exit was executed.
type ExitReason string

const (
	ExitReasonStop      ExitReason = "stop"
	ExitReasonTarget    ExitReason = "target"
	ExitReasonBreakeven ExitReason = "breakeven"
	ExitReasonTime      ExitReason = "time"
	ExitReasonFoldEnd   ExitReason = "fold_end"
	ExitReasonSignal    ExitReason = "signal"
)

func (r ExitReason) Validate() error {
	switch r {
	case ExitReasonStop, ExitReasonTarget, ExitReasonBreakeven, ExitReasonTime, ExitReasonFoldEnd, ExitReasonSignal:
		return nil
	default:
		return fmt.Errorf("execution: invalid exit reason %q", r)
	}
}

// FillAudit is the cost and timing evidence common to every fill.
type FillAudit struct {
	FillID           string                   `json:"fill_id"`
	IntentID         string                   `json:"intent_id"`
	SignalID         string                   `json:"signal_id"`
	Strategy         protocolv2.StrategyRef   `json:"strategy"`
	Symbol           protocolv2.Symbol        `json:"symbol"`
	Side             Side                     `json:"side"`
	SourceCandleTime time.Time                `json:"source_candle_time"`
	FillTime         time.Time                `json:"fill_time"`
	ReferencePrice   float64                  `json:"reference_price"`
	Price            float64                  `json:"price"`
	Quantity         float64                  `json:"quantity"`
	Commission       float64                  `json:"commission"`
	Slippage         float64                  `json:"slippage"`
	CostProfile      protocolv2.CostProfileID `json:"cost_profile"`
	Audit            map[string]float64       `json:"audit,omitempty"`
}

func (f FillAudit) Validate() error {
	if f.FillID == "" || f.IntentID == "" || f.SignalID == "" {
		return fmt.Errorf("execution: fill, intent, and signal ids are required")
	}
	if err := f.Strategy.Validate(); err != nil {
		return err
	}
	if err := f.Symbol.Validate(); err != nil {
		return err
	}
	if err := f.Side.Validate(); err != nil {
		return err
	}
	if err := validateUTC("source candle time", f.SourceCandleTime); err != nil {
		return err
	}
	if err := validateUTC("fill time", f.FillTime); err != nil {
		return err
	}
	if !f.FillTime.After(f.SourceCandleTime) {
		return fmt.Errorf("execution: fill time must be after source candle time")
	}
	if err := validatePrice("reference price", f.ReferencePrice); err != nil {
		return err
	}
	if err := validatePrice("fill price", f.Price); err != nil {
		return err
	}
	if err := validateQuantity("fill quantity", f.Quantity); err != nil {
		return err
	}
	if err := validateNonNegativeFee("commission", f.Commission); err != nil {
		return err
	}
	if err := validateNonNegativeFee("slippage", f.Slippage); err != nil {
		return err
	}
	if err := f.CostProfile.Validate(); err != nil {
		return err
	}
	return validateDiagnostics(f.Audit)
}

type EntryFill struct {
	FillAudit
	PositionID string `json:"position_id"`
}

func (f EntryFill) Validate() error {
	if f.PositionID == "" {
		return fmt.Errorf("execution: position id is required")
	}
	return f.FillAudit.Validate()
}

type PartialExitFill struct {
	FillAudit
	PositionID string     `json:"position_id"`
	Reason     ExitReason `json:"reason"`
}

func (f PartialExitFill) Validate() error {
	if f.PositionID == "" {
		return fmt.Errorf("execution: position id is required")
	}
	if err := f.Reason.Validate(); err != nil {
		return err
	}
	return f.FillAudit.Validate()
}

type FinalExitFill struct {
	FillAudit
	PositionID string     `json:"position_id"`
	Reason     ExitReason `json:"reason"`
}

func (f FinalExitFill) Validate() error {
	if f.PositionID == "" {
		return fmt.Errorf("execution: position id is required")
	}
	if err := f.Reason.Validate(); err != nil {
		return err
	}
	return f.FillAudit.Validate()
}

type PositionStatus string

const (
	PositionOpen   PositionStatus = "open"
	PositionClosed PositionStatus = "closed"
)

func (s PositionStatus) Validate() error {
	if s != PositionOpen && s != PositionClosed {
		return fmt.Errorf("execution: invalid position status %q", s)
	}
	return nil
}

// PositionState is the quantity state of one standalone long position.
type PositionState struct {
	PositionID        string                 `json:"position_id"`
	Strategy          protocolv2.StrategyRef `json:"strategy"`
	Symbol            protocolv2.Symbol      `json:"symbol"`
	Side              Side                   `json:"side"`
	Status            PositionStatus         `json:"status"`
	OpenedAt          time.Time              `json:"opened_at"`
	InitialQuantity   float64                `json:"initial_quantity"`
	RemainingQuantity float64                `json:"remaining_quantity"`
	AverageEntryPrice float64                `json:"average_entry_price"`
	Stop              float64                `json:"stop"`
}

func (p PositionState) Validate() error {
	if p.PositionID == "" {
		return fmt.Errorf("execution: position id is required")
	}
	if err := p.Strategy.Validate(); err != nil {
		return err
	}
	if err := p.Symbol.Validate(); err != nil {
		return err
	}
	if err := p.Side.Validate(); err != nil {
		return err
	}
	if err := p.Status.Validate(); err != nil {
		return err
	}
	if err := validateUTC("opened at", p.OpenedAt); err != nil {
		return err
	}
	if err := validateQuantity("initial quantity", p.InitialQuantity); err != nil {
		return err
	}
	if err := validateNonNegativeQuantity("remaining quantity", p.RemainingQuantity); err != nil {
		return err
	}
	if p.RemainingQuantity > p.InitialQuantity {
		return fmt.Errorf("execution: remaining quantity exceeds initial quantity")
	}
	if p.Status == PositionOpen && p.RemainingQuantity == 0 {
		return fmt.Errorf("execution: open position must have remaining quantity")
	}
	if p.Status == PositionClosed && p.RemainingQuantity != 0 {
		return fmt.Errorf("execution: closed position must have zero remaining quantity")
	}
	if err := validatePrice("average entry price", p.AverageEntryPrice); err != nil {
		return err
	}
	return validatePrice("stop", p.Stop)
}

type TradeStatus string

const (
	TradeOpen   TradeStatus = "open"
	TradeClosed TradeStatus = "closed"
)

func (s TradeStatus) Validate() error {
	if s != TradeOpen && s != TradeClosed {
		return fmt.Errorf("execution: invalid trade status %q", s)
	}
	return nil
}

// TradeState links all fills for a position and preserves partial exits.
type TradeState struct {
	TradeID      string            `json:"trade_id"`
	PositionID   string            `json:"position_id"`
	Status       TradeStatus       `json:"status"`
	Entry        EntryFill         `json:"entry"`
	PartialExits []PartialExitFill `json:"partial_exits,omitempty"`
	FinalExit    *FinalExitFill    `json:"final_exit,omitempty"`
}

func (t TradeState) Validate() error {
	if t.TradeID == "" || t.PositionID == "" {
		return fmt.Errorf("execution: trade and position ids are required")
	}
	if err := t.Status.Validate(); err != nil {
		return err
	}
	if err := t.Entry.Validate(); err != nil {
		return err
	}
	if t.Entry.PositionID != t.PositionID {
		return fmt.Errorf("execution: entry position id does not match trade")
	}
	// Quantities are protocol-rounded to eight decimal places. Re-round each
	// accumulation so binary floating-point noise cannot turn a reconciled
	// partial exit plus final exit into an apparent over-exit.
	exited := float64(0)
	for _, fill := range t.PartialExits {
		if err := fill.Validate(); err != nil {
			return err
		}
		if fill.PositionID != t.PositionID {
			return fmt.Errorf("execution: partial exit position id does not match trade")
		}
		exited = protocolv2.RoundQuantity(exited + fill.Quantity)
	}
	if t.FinalExit != nil {
		if err := t.FinalExit.Validate(); err != nil {
			return err
		}
		if t.FinalExit.PositionID != t.PositionID {
			return fmt.Errorf("execution: final exit position id does not match trade")
		}
		exited = protocolv2.RoundQuantity(exited + t.FinalExit.Quantity)
	}
	if exited-t.Entry.Quantity > quantityTolerance(exited, t.Entry.Quantity) {
		return fmt.Errorf("execution: exited quantity exceeds entry quantity")
	}
	if t.Status == TradeOpen && t.FinalExit != nil {
		return fmt.Errorf("execution: open trade cannot have final exit")
	}
	if t.Status == TradeClosed && t.FinalExit == nil {
		return fmt.Errorf("execution: closed trade requires final exit")
	}
	if t.Status == TradeClosed && !quantitiesReconcile(exited, t.Entry.Quantity) {
		return fmt.Errorf("execution: closed trade quantities must reconcile")
	}
	return nil
}

// EquitySnapshot records mark-to-market account evidence after one event/bar.
type EquitySnapshot struct {
	Time              time.Time `json:"time"`
	Cash              float64   `json:"cash"`
	OpenPositionValue float64   `json:"open_position_value"`
	RealizedPnL       float64   `json:"realized_pnl"`
	UnrealizedPnL     float64   `json:"unrealized_pnl"`
	CommissionCosts   float64   `json:"commission_costs"`
	SlippageCosts     float64   `json:"slippage_costs"`
	TotalEquity       float64   `json:"total_equity"`
	UnderwaterPercent float64   `json:"underwater_percent"`
}

func (s EquitySnapshot) Validate() error {
	if err := validateUTC("snapshot time", s.Time); err != nil {
		return err
	}
	for name, value := range map[string]float64{
		"cash": s.Cash, "open position value": s.OpenPositionValue,
		"realized pnl": s.RealizedPnL, "unrealized pnl": s.UnrealizedPnL,
		"total equity": s.TotalEquity,
	} {
		if err := validateRoundedFee(name, value); err != nil {
			return err
		}
	}
	if s.Cash < 0 || s.OpenPositionValue < 0 || s.TotalEquity < 0 {
		return fmt.Errorf("execution: cash, open position value, and total equity must not be negative")
	}
	if err := validateNonNegativeFee("commission costs", s.CommissionCosts); err != nil {
		return err
	}
	if err := validateNonNegativeFee("slippage costs", s.SlippageCosts); err != nil {
		return err
	}
	if s.UnderwaterPercent > 0 || !isRounded(s.UnderwaterPercent, protocolv2.RoundMetric) {
		return fmt.Errorf("execution: underwater percent must be finite, non-positive, and metric-rounded")
	}
	if s.TotalEquity != protocolv2.RoundFee(s.Cash+s.OpenPositionValue) {
		return fmt.Errorf("execution: total equity must equal cash plus open position value")
	}
	return nil
}

// SignalRejection records a deterministic reason an attempted signal did not
// become an order or fill.
type SignalRejection struct {
	SignalID    string                     `json:"signal_id"`
	Strategy    protocolv2.StrategyRef     `json:"strategy"`
	Symbol      protocolv2.Symbol          `json:"symbol"`
	OccurredAt  time.Time                  `json:"occurred_at"`
	Reason      protocolv2.RejectionReason `json:"reason"`
	Diagnostics map[string]float64         `json:"diagnostics,omitempty"`
}

func (r SignalRejection) Validate() error {
	if r.SignalID == "" {
		return fmt.Errorf("execution: signal id is required")
	}
	if err := r.Strategy.Validate(); err != nil {
		return err
	}
	if err := r.Symbol.Validate(); err != nil {
		return err
	}
	if err := validateUTC("rejection time", r.OccurredAt); err != nil {
		return err
	}
	if err := r.Reason.Validate(); err != nil {
		return err
	}
	return validateDiagnostics(r.Diagnostics)
}

func validateUTC(name string, value time.Time) error {
	if value.IsZero() || value.Location() != time.UTC {
		return fmt.Errorf("execution: %s must be a non-zero UTC time", name)
	}
	return nil
}

func validatePrice(name string, value float64) error {
	if value <= 0 || !isRounded(value, protocolv2.RoundPrice) {
		return fmt.Errorf("execution: %s must be finite, positive, and price-rounded", name)
	}
	return nil
}

func validateQuantity(name string, value float64) error {
	rounded := protocolv2.RoundQuantity(value)
	if value <= 0 || !isQuantityRounded(value, rounded) {
		return fmt.Errorf("execution: %s must be finite, positive, and quantity-rounded (value=%.17g rounded=%.17g)", name, value, rounded)
	}
	return nil
}

func validateNonNegativeQuantity(name string, value float64) error {
	rounded := protocolv2.RoundQuantity(value)
	if value < 0 || !isQuantityRounded(value, rounded) {
		return fmt.Errorf("execution: %s must be finite, non-negative, and quantity-rounded (value=%.17g rounded=%.17g)", name, value, rounded)
	}
	return nil
}

func isQuantityRounded(value, rounded float64) bool {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	scale := math.Max(math.Abs(value), math.Abs(rounded))
	if scale == 0 {
		return true
	}
	ulp := math.Nextafter(scale, math.Inf(1)) - scale
	return math.Abs(value-rounded) <= 8*ulp
}

func quantitiesReconcile(left, right float64) bool {
	return math.Abs(left-right) <= quantityTolerance(left, right)
}

// quantityTolerance accounts for float64 spacing at large token quantities.
// At SHIB-scale positions a nominal 1e-8 quantity tick is smaller than one
// representable float64 step, so exact equality would reject reconciled fills.
func quantityTolerance(left, right float64) float64 {
	scale := math.Max(math.Abs(left), math.Abs(right))
	if scale == 0 {
		return 0
	}
	ulp := math.Nextafter(scale, math.Inf(1)) - scale
	return math.Max(1e-8, 8*ulp)
}

func validateNonNegativeFee(name string, value float64) error {
	if value < 0 || !isRounded(value, protocolv2.RoundFee) {
		return fmt.Errorf("execution: %s must be finite, non-negative, and fee-rounded", name)
	}
	return nil
}

func validateRoundedFee(name string, value float64) error {
	if !isRounded(value, protocolv2.RoundFee) {
		return fmt.Errorf("execution: %s must be finite and fee-rounded", name)
	}
	return nil
}

func isRounded(value float64, round func(float64) float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value == round(value)
}

func validateDiagnostics(values map[string]float64) error {
	for key, value := range values {
		if key == "" {
			return fmt.Errorf("execution: diagnostic key is required")
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("execution: diagnostic %q must be finite", key)
		}
	}
	return nil
}

func sortedTargets(targets []Target) []Target {
	out := append([]Target(nil), targets...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

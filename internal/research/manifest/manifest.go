package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

const ProtocolVersion = "protocol-v2"

var coreStrategyCodes = map[protocolv2.StrategyCode]struct{}{
	"fib-pullback-trend-v1":              {},
	"nr7-trend-breakout-v1":              {},
	"volatility-compression-breakout-v1": {},
	"breakout-retest-long-v2":            {},
}

var researchV3StrategyCodes = map[protocolv2.StrategyCode]struct{}{
	"volatility-compression-breakout-v2": {},
	"mean-reversion-v1":                  {},
	"daily-low-zone-v1":                  {},
}

var dailyLowZoneV11StrategyCodes = map[protocolv2.StrategyCode]struct{}{
	"daily-low-zone-v1": {},
}

var dailyLowZoneV12StrategyCodes = map[protocolv2.StrategyCode]struct{}{
	"daily-low-zone-v1": {},
}

var dailyLowZoneV13StrategyCodes = map[protocolv2.StrategyCode]struct{}{
	"daily-low-zone-v1": {},
}

var rsiMeanReversionV1StrategyCodes = map[protocolv2.StrategyCode]struct{}{
	"rsi-mean-reversion-long-v1": {},
}

// ValidateCoreStrategyCodes applies the deliberately narrow core-validation
// scope after normal manifest validation. It rejects strategies reserved for
// follow-up research changes without restricting non-core manifest consumers.
func ValidateCoreStrategyCodes(strategies []Strategy) error {
	return validateStrategySuite("core validation", strategies, coreStrategyCodes)
}

// ValidateResearchV3StrategyCodes keeps the next experiment isolated from the
// completed core study. A manifest contains exactly one approved suite.
func ValidateResearchV3StrategyCodes(strategies []Strategy) error {
	return validateStrategySuite("research-v3", strategies, researchV3StrategyCodes)
}

func ValidateDailyLowZoneV11StrategyCodes(strategies []Strategy) error {
	return validateStrategySuite("daily-low-zone-v1_1", strategies, dailyLowZoneV11StrategyCodes)
}

func ValidateDailyLowZoneV12StrategyCodes(strategies []Strategy) error {
	return validateStrategySuite("daily-low-zone-v1_2", strategies, dailyLowZoneV12StrategyCodes)
}

func ValidateDailyLowZoneV13StrategyCodes(strategies []Strategy) error {
	return validateStrategySuite("daily-low-zone-v1_3", strategies, dailyLowZoneV13StrategyCodes)
}

func ValidateRSIMeanReversionV1StrategyCodes(strategies []Strategy) error {
	return validateStrategySuite("rsi-mean-reversion-v1", strategies, rsiMeanReversionV1StrategyCodes)
}

// ValidateSupportedStrategyCodes accepts one complete protocol suite, never a
// mixture of historical core and new research candidates.
func ValidateSupportedStrategyCodes(strategies []Strategy) error {
	if err := ValidateCoreStrategyCodes(strategies); err == nil {
		return nil
	}
	if err := ValidateResearchV3StrategyCodes(strategies); err == nil {
		return nil
	}
	if err := ValidateDailyLowZoneV11StrategyCodes(strategies); err == nil {
		return nil
	}
	if err := ValidateDailyLowZoneV12StrategyCodes(strategies); err == nil {
		return nil
	}
	if err := ValidateDailyLowZoneV13StrategyCodes(strategies); err == nil {
		return nil
	}
	return ValidateRSIMeanReversionV1StrategyCodes(strategies)
}

func validateStrategySuite(name string, strategies []Strategy, allowed map[protocolv2.StrategyCode]struct{}) error {
	if len(strategies) != len(allowed) {
		return fmt.Errorf("manifest: %s requires exactly %d strategies", name, len(allowed))
	}
	seen := make(map[protocolv2.StrategyCode]struct{}, len(strategies))
	for _, strategy := range strategies {
		if _, ok := allowed[strategy.Ref.Code]; !ok {
			return fmt.Errorf("manifest: strategy %q is outside %s scope", strategy.Ref.Code, name)
		}
		if _, duplicate := seen[strategy.Ref.Code]; duplicate {
			return fmt.Errorf("manifest: duplicate %s strategy code %q", name, strategy.Ref.Code)
		}
		seen[strategy.Ref.Code] = struct{}{}
	}
	return nil
}

type GapPolicy string

const (
	GapReject            GapPolicy = "reject-signal"
	GapFillNextAvailable GapPolicy = "fill-next-available"
)

func (p GapPolicy) Validate() error {
	if p != GapReject && p != GapFillNextAvailable {
		return fmt.Errorf("manifest: invalid gap policy %q", p)
	}
	return nil
}

type IntrabarPolicy string

const IntrabarStopFirst IntrabarPolicy = "stop-first"

func (p IntrabarPolicy) Validate() error {
	if p != IntrabarStopFirst {
		return fmt.Errorf("manifest: invalid intrabar policy %q", p)
	}
	return nil
}

// SourceRevision identifies the exact source state used by an experiment.
type SourceRevision struct {
	GitRevision string `json:"git_revision"`
	Dirty       bool   `json:"dirty"`
}

type SymbolSnapshot struct {
	Symbol       protocolv2.Symbol    `json:"symbol"`
	CandleSHA256 protocolv2.SHA256Hex `json:"candle_sha256"`
}

type UniverseSnapshot struct {
	Name       string                        `json:"name"`
	Provenance protocolv2.UniverseProvenance `json:"provenance"`
	Exchange   string                        `json:"exchange"`
	Spot       bool                          `json:"spot"`
	QuoteAsset string                        `json:"quote_asset"`
	Symbols    []SymbolSnapshot              `json:"symbols"`
	Exclusions []protocolv2.Symbol           `json:"exclusions,omitempty"`
}

// ParameterCandidate is one immutable strategy grid point. Parameters are
// deliberately JSON values to let strategies own their parameter schema.
type ParameterCandidate struct {
	ID     protocolv2.ParameterCandidateID `json:"id"`
	Values map[string]any                  `json:"values"`
}

type Strategy struct {
	Ref           protocolv2.StrategyRef `json:"ref"`
	Timeframe     protocolv2.Timeframe   `json:"timeframe"`
	WarmupBars    int                    `json:"warmup_bars"`
	DefaultParams map[string]any         `json:"default_params"`
	Grid          []ParameterCandidate   `json:"grid"`
}

type Schedule struct {
	Train         protocolv2.TimeRange `json:"train"`
	Test          protocolv2.TimeRange `json:"test"`
	FoldStep      time.Duration        `json:"fold_step"`
	LockedHoldout protocolv2.TimeRange `json:"locked_holdout"`
}

type ExecutionProfile struct {
	ID             protocolv2.CostProfileID `json:"id"`
	CommissionBPS  float64                  `json:"commission_bps"`
	SlippageBPS    float64                  `json:"slippage_bps"`
	GapPolicy      GapPolicy                `json:"gap_policy"`
	IntrabarPolicy IntrabarPolicy           `json:"intrabar_policy"`
}

type StandaloneRisk struct {
	SizingProfile       protocolv2.SizingProfileID `json:"sizing_profile"`
	InitialEquity       float64                    `json:"initial_equity"`
	RiskPerTradePercent float64                    `json:"risk_per_trade_percent"`
	MaxNotionalPercent  float64                    `json:"max_notional_percent"`
}

type Gates struct {
	MinAggregateTrades        int     `json:"min_aggregate_trades"`
	MinEligibleSymbols        int     `json:"min_eligible_symbols"`
	MinDevelopmentFolds       int     `json:"min_development_folds"`
	MinPositiveFoldFraction   float64 `json:"min_positive_fold_fraction"`
	MinProfitFactor           float64 `json:"min_profit_factor"`
	RequirePositiveExpectancy bool    `json:"require_positive_expectancy"`
	MaxMedianDrawdownPercent  float64 `json:"max_median_drawdown_percent"`
	MaxContributionPercent    float64 `json:"max_contribution_percent"`
	RequireStressPositive     bool    `json:"require_stress_positive"`
	RequireParameterStability bool    `json:"require_parameter_stability"`
	RequireHoldoutConsistency bool    `json:"require_holdout_consistency"`
}

// Manifest contains every input which can change a protocol-v2 experiment.
// ID and Hash are derived from the canonical payload and never supplied by a
// caller as identity inputs.
type Manifest struct {
	SchemaVersion   string                  `json:"schema_version"`
	ProtocolVersion string                  `json:"protocol_version"`
	ID              protocolv2.ExperimentID `json:"id"`
	Hash            protocolv2.SHA256Hex    `json:"hash"`
	Cutoff          time.Time               `json:"cutoff"`
	Source          SourceRevision          `json:"source"`
	Seed            uint64                  `json:"seed"`
	Universe        UniverseSnapshot        `json:"universe"`
	Strategies      []Strategy              `json:"strategies"`
	Schedule        Schedule                `json:"schedule"`
	Execution       ExecutionProfile        `json:"execution"`
	Risk            StandaloneRisk          `json:"standalone_risk"`
	Gates           Gates                   `json:"gates"`
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != protocolv2.ManifestSchemaVersion {
		return fmt.Errorf("manifest: schema_version must be %q", protocolv2.ManifestSchemaVersion)
	}
	if m.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("manifest: protocol_version must be %q", ProtocolVersion)
	}
	if m.Cutoff.IsZero() || m.Cutoff.Location() != time.UTC {
		return fmt.Errorf("manifest: cutoff must be a non-zero UTC time")
	}
	if strings.TrimSpace(m.Source.GitRevision) == "" {
		return fmt.Errorf("manifest: source git revision is required")
	}
	if err := validateUniverse(m.Universe); err != nil {
		return err
	}
	if len(m.Strategies) == 0 {
		return fmt.Errorf("manifest: at least one strategy is required")
	}
	seen := make(map[string]bool)
	for _, s := range m.Strategies {
		if err := validateStrategy(s); err != nil {
			return err
		}
		if seen[s.Ref.String()] {
			return fmt.Errorf("manifest: duplicate strategy %s", s.Ref)
		}
		seen[s.Ref.String()] = true
	}
	if err := validateSchedule(m.Schedule, m.Cutoff); err != nil {
		return err
	}
	if err := validateExecution(m.Execution); err != nil {
		return err
	}
	if err := validateRisk(m.Risk); err != nil {
		return err
	}
	return validateGates(m.Gates)
}

func validateUniverse(u UniverseSnapshot) error {
	if strings.TrimSpace(u.Name) == "" || strings.TrimSpace(u.Exchange) == "" || strings.TrimSpace(u.QuoteAsset) == "" {
		return fmt.Errorf("manifest: universe name, exchange, and quote_asset are required")
	}
	if !u.Spot {
		return fmt.Errorf("manifest: universe must be spot")
	}
	if err := u.Provenance.Validate(); err != nil {
		return err
	}
	if len(u.Symbols) == 0 {
		return fmt.Errorf("manifest: universe symbols are required")
	}
	seen := map[protocolv2.Symbol]bool{}
	for _, s := range u.Symbols {
		if err := s.Symbol.Validate(); err != nil {
			return err
		}
		if err := s.CandleSHA256.Validate(); err != nil {
			return err
		}
		if seen[s.Symbol] {
			return fmt.Errorf("manifest: duplicate universe symbol %q", s.Symbol)
		}
		seen[s.Symbol] = true
	}
	for _, s := range u.Exclusions {
		if err := s.Validate(); err != nil {
			return err
		}
		if seen[s] {
			return fmt.Errorf("manifest: excluded symbol %q is in universe", s)
		}
	}
	return nil
}

func validateStrategy(s Strategy) error {
	if err := s.Ref.Validate(); err != nil {
		return err
	}
	if err := s.Timeframe.Validate(); err != nil {
		return err
	}
	if s.WarmupBars < 0 {
		return fmt.Errorf("manifest: warmup bars cannot be negative")
	}
	if s.DefaultParams == nil {
		return fmt.Errorf("manifest: default parameters are required")
	}
	if len(s.Grid) == 0 || len(s.Grid) > 30 {
		return fmt.Errorf("manifest: strategy %s grid must contain 1 to 30 candidates", s.Ref)
	}
	seen := map[protocolv2.ParameterCandidateID]bool{}
	for _, c := range s.Grid {
		if err := c.ID.Validate(); err != nil {
			return err
		}
		if c.Values == nil {
			return fmt.Errorf("manifest: candidate %q values are required", c.ID)
		}
		if seen[c.ID] {
			return fmt.Errorf("manifest: duplicate candidate %q", c.ID)
		}
		seen[c.ID] = true
	}
	return nil
}

func validateSchedule(s Schedule, cutoff time.Time) error {
	for _, named := range []struct {
		name string
		r    protocolv2.TimeRange
	}{{"train", s.Train}, {"test", s.Test}, {"locked_holdout", s.LockedHoldout}} {
		if err := named.r.Validate(); err != nil {
			return err
		}
		if named.r.Duration() <= 0 {
			return fmt.Errorf("manifest: %s range must not be empty", named.name)
		}
	}
	if s.FoldStep <= 0 {
		return fmt.Errorf("manifest: fold step must be positive")
	}
	if s.Train.End.After(s.Test.Start) || s.Test.End.After(s.LockedHoldout.Start) {
		return fmt.Errorf("manifest: train, test, and locked holdout must be chronological and non-overlapping")
	}
	if s.LockedHoldout.End.After(cutoff) {
		return fmt.Errorf("manifest: locked holdout ends after cutoff")
	}
	return nil
}

func validateExecution(e ExecutionProfile) error {
	if err := e.ID.Validate(); err != nil {
		return err
	}
	if !finiteNonNegative(e.CommissionBPS) || !finiteNonNegative(e.SlippageBPS) {
		return fmt.Errorf("manifest: commission and slippage must be finite non-negative values")
	}
	if err := e.GapPolicy.Validate(); err != nil {
		return err
	}
	return e.IntrabarPolicy.Validate()
}

func validateRisk(r StandaloneRisk) error {
	if err := r.SizingProfile.Validate(); err != nil {
		return err
	}
	if !finitePositive(r.InitialEquity) || !finitePositive(r.RiskPerTradePercent) || !finitePositive(r.MaxNotionalPercent) || r.RiskPerTradePercent > 100 || r.MaxNotionalPercent > 100 {
		return fmt.Errorf("manifest: invalid standalone risk profile")
	}
	return nil
}

func validateGates(g Gates) error {
	if g.MinAggregateTrades <= 0 || g.MinEligibleSymbols <= 0 || g.MinDevelopmentFolds <= 0 || !finitePositive(g.MinPositiveFoldFraction) || g.MinPositiveFoldFraction > 1 || !finitePositive(g.MinProfitFactor) || !finitePositive(g.MaxMedianDrawdownPercent) || g.MaxMedianDrawdownPercent > 100 || !finitePositive(g.MaxContributionPercent) || g.MaxContributionPercent > 100 {
		return fmt.Errorf("manifest: invalid gate thresholds")
	}
	return nil
}

func finitePositive(v float64) bool    { return !math.IsNaN(v) && !math.IsInf(v, 0) && v > 0 }
func finiteNonNegative(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 }

// CanonicalJSON produces a deterministic identity payload. Derived ID and Hash
// are omitted and semantically unordered manifest lists are sorted.
func (m Manifest) CanonicalJSON() ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	m = m.canonicalized()
	m.ID, m.Hash = "", ""
	return json.Marshal(m)
}

func (m Manifest) canonicalized() Manifest {
	m.Universe.Symbols = append([]SymbolSnapshot(nil), m.Universe.Symbols...)
	m.Universe.Exclusions = append([]protocolv2.Symbol(nil), m.Universe.Exclusions...)
	m.Strategies = append([]Strategy(nil), m.Strategies...)
	for i := range m.Strategies {
		m.Strategies[i].Grid = append([]ParameterCandidate(nil), m.Strategies[i].Grid...)
	}
	m.Cutoff = m.Cutoff.UTC()
	sort.Slice(m.Universe.Symbols, func(i, j int) bool { return m.Universe.Symbols[i].Symbol < m.Universe.Symbols[j].Symbol })
	sort.Slice(m.Universe.Exclusions, func(i, j int) bool { return m.Universe.Exclusions[i] < m.Universe.Exclusions[j] })
	sort.Slice(m.Strategies, func(i, j int) bool { return m.Strategies[i].Ref.String() < m.Strategies[j].Ref.String() })
	for i := range m.Strategies {
		sort.Slice(m.Strategies[i].Grid, func(a, b int) bool { return m.Strategies[i].Grid[a].ID < m.Strategies[i].Grid[b].ID })
	}
	return m
}

// Identity derives the full digest and a valid human-safe experiment ID.
func (m Manifest) Identity() (protocolv2.ExperimentID, protocolv2.SHA256Hex, error) {
	b, err := m.CanonicalJSON()
	if err != nil {
		return "", "", err
	}
	sum := sha256.Sum256(b)
	hash := protocolv2.SHA256Hex(hex.EncodeToString(sum[:]))
	return protocolv2.ExperimentID("exp-" + string(hash[:24])), hash, nil
}

// Freeze assigns the identity derived from the manifest's reproducibility inputs.
func (m *Manifest) Freeze() error {
	id, hash, err := m.Identity()
	if err != nil {
		return err
	}
	m.ID, m.Hash = id, hash
	return nil
}

// MarshalCanonical emits a frozen JSON document after verifying its identity.
func (m Manifest) MarshalCanonical() ([]byte, error) {
	id, hash, err := m.Identity()
	if err != nil {
		return nil, err
	}
	if m.ID != id || m.Hash != hash {
		return nil, fmt.Errorf("manifest: supplied identity does not match manifest content")
	}
	m = m.canonicalized()
	return json.Marshal(m)
}

// Decode parses one manifest while rejecting unknown JSON fields and a stale or
// manually supplied identity.
func Decode(data []byte) (Manifest, error) {
	var m Manifest
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("manifest: decode: %w", err)
	}
	var trailing any
	if err := d.Decode(&trailing); err != io.EOF {
		return Manifest{}, fmt.Errorf("manifest: decode: multiple JSON values")
	}
	id, hash, err := m.Identity()
	if err != nil {
		return Manifest{}, err
	}
	if m.ID != id || m.Hash != hash {
		return Manifest{}, fmt.Errorf("manifest: identity does not match content")
	}
	return m, nil
}

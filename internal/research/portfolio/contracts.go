// Package portfolio implements causal shared-capital portfolio research.
package portfolio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/manifest"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

const (
	ManifestSchemaVersion = "portfolio.manifest.v1"
	ReportSchemaVersion   = "portfolio.report.v1"
	StrategyCode          = "relative-strength-long-v1"
	StrategyVersion       = "v1.0.0"
)

// RegimeMode controls which frozen market filters can enable relative-strength
// allocation. It is an experiment input, never a runtime tuning knob.
type RegimeMode string

const (
	RegimeModeBoth    RegimeMode = "both"
	RegimeModeBTCEMA  RegimeMode = "btc-ema"
	RegimeModeBreadth RegimeMode = "breadth"
	RegimeModeNone    RegimeMode = "none"
)

func (m RegimeMode) Validate() error {
	switch m {
	case "", RegimeModeBoth, RegimeModeBTCEMA, RegimeModeBreadth, RegimeModeNone:
		return nil
	default:
		return fmt.Errorf("portfolio: invalid regime mode %q", m)
	}
}

func (m RegimeMode) normalized() RegimeMode {
	if m == "" {
		return RegimeModeBoth
	}
	return m
}

// EntryMode is frozen independently from the market regime so a pullback
// hypothesis cannot be mistaken for the original weekly-open momentum entry.
type EntryMode string

const (
	EntryModeWeeklyOpen    EntryMode = "weekly-open"
	EntryModeTrendPullback EntryMode = "trend-pullback"
)

func (m EntryMode) Validate() error {
	switch m {
	case "", EntryModeWeeklyOpen, EntryModeTrendPullback:
		return nil
	default:
		return fmt.Errorf("portfolio: invalid entry mode %q", m)
	}
}

func (m EntryMode) normalized() EntryMode {
	if m == "" {
		return EntryModeWeeklyOpen
	}
	return m
}

type SignalArtifactRef struct {
	Path       string                          `json:"path"`
	SHA256     protocolv2.SHA256Hex            `json:"sha256"`
	Experiment protocolv2.ExperimentID         `json:"experiment_id"`
	Strategy   protocolv2.StrategyRef          `json:"strategy"`
	Candidate  protocolv2.ParameterCandidateID `json:"candidate"`
	Decision   string                          `json:"decision"`
}

type CostProfile struct {
	CommissionBPS float64 `json:"commission_bps"`
	SlippageBPS   float64 `json:"slippage_bps"`
}

type Limits struct {
	InitialCapital      float64 `json:"initial_capital"`
	RiskPerTradePercent float64 `json:"risk_per_trade_percent"`
	MaxPositionPercent  float64 `json:"max_position_percent"`
	MaxPositions        int     `json:"max_positions"`
	MaxAggregateRiskPct float64 `json:"max_aggregate_risk_percent"`
}

type RelativeStrengthConfig struct {
	ReturnLookbackDays  int          `json:"return_lookback_days"`
	VolatilityDays      int          `json:"volatility_days"`
	ATRDays             int          `json:"atr_days"`
	StopATR             float64      `json:"stop_atr"`
	TopK                int          `json:"top_k"`
	ExitRank            int          `json:"exit_rank"`
	BTCEMADays          int          `json:"btc_ema_days"`
	MinPositiveBreadth  float64      `json:"min_positive_breadth"`
	RebalanceWeekday    time.Weekday `json:"rebalance_weekday"`
	RegimeMode          RegimeMode   `json:"regime_mode"`
	EntryMode           EntryMode    `json:"entry_mode"`
	PullbackEMADays     int          `json:"pullback_ema_days"`
	MaxEntryDistanceATR float64      `json:"max_entry_distance_atr"`
}

type Gates struct {
	MinNetReturn           float64 `json:"min_net_return"`
	MaxDrawdown            float64 `json:"max_drawdown"`
	MinExcessVsBTC         float64 `json:"min_excess_vs_btc"`
	MinExcessVsEqualWeight float64 `json:"min_excess_vs_equal_weight"`
	MaxContribution        float64 `json:"max_contribution"`
	RequireStressPositive  bool    `json:"require_stress_positive"`
}

type Manifest struct {
	SchemaVersion          string                    `json:"schema_version"`
	ID                     protocolv2.ExperimentID   `json:"id"`
	Hash                   protocolv2.SHA256Hex      `json:"hash"`
	ImplementationRevision string                    `json:"implementation_revision"`
	SourceExperiment       protocolv2.ExperimentID   `json:"source_experiment"`
	SourceManifestHash     protocolv2.SHA256Hex      `json:"source_manifest_hash"`
	Diagnostic             bool                      `json:"diagnostic"`
	SignalArtifacts        []SignalArtifactRef       `json:"signal_artifacts,omitempty"`
	Universe               manifest.UniverseSnapshot `json:"universe"`
	Range                  protocolv2.TimeRange      `json:"range"`
	BaseCosts              CostProfile               `json:"base_costs"`
	StressCosts            CostProfile               `json:"stress_costs"`
	Limits                 Limits                    `json:"limits"`
	RelativeStrength       RelativeStrengthConfig    `json:"relative_strength"`
	Gates                  Gates                     `json:"gates"`
}

func DefaultManifest(source manifest.Manifest, revision string, diagnostic bool, regimeMode RegimeMode, entryMode EntryMode) (Manifest, error) {
	regimeMode = regimeMode.normalized()
	entryMode = entryMode.normalized()
	m := Manifest{
		SchemaVersion: ManifestSchemaVersion, ImplementationRevision: revision,
		SourceExperiment: source.ID, SourceManifestHash: source.Hash, Diagnostic: diagnostic,
		Universe:         source.Universe,
		Range:            protocolv2.TimeRange{Start: source.Schedule.Test.Start, End: source.Schedule.LockedHoldout.Start},
		BaseCosts:        CostProfile{CommissionBPS: source.Execution.CommissionBPS, SlippageBPS: source.Execution.SlippageBPS},
		StressCosts:      CostProfile{CommissionBPS: source.Execution.CommissionBPS, SlippageBPS: 15},
		Limits:           Limits{InitialCapital: source.Risk.InitialEquity, RiskPerTradePercent: 1, MaxPositionPercent: 20, MaxPositions: 5, MaxAggregateRiskPct: 5},
		RelativeStrength: RelativeStrengthConfig{ReturnLookbackDays: 90, VolatilityDays: 30, ATRDays: 20, StopATR: 2, TopK: 5, ExitRank: 10, BTCEMADays: 200, MinPositiveBreadth: .5, RebalanceWeekday: time.Monday, RegimeMode: regimeMode, EntryMode: entryMode, PullbackEMADays: 20, MaxEntryDistanceATR: .5},
		Gates:            Gates{MinNetReturn: 0, MaxDrawdown: .25, MinExcessVsBTC: -.05, MinExcessVsEqualWeight: -.05, MaxContribution: .4, RequireStressPositive: true},
	}
	if err := m.freeze(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func (m *Manifest) freeze() error {
	m.ID, m.Hash = "", ""
	if err := m.Validate(); err != nil {
		return err
	}
	hash, err := m.payloadHash()
	if err != nil {
		return err
	}
	m.Hash = hash
	m.ID = protocolv2.ExperimentID("portfolio-" + string(hash[:24]))
	return m.Validate()
}

func (m Manifest) Validate() error {
	if m.SchemaVersion != ManifestSchemaVersion {
		return fmt.Errorf("portfolio: invalid manifest schema")
	}
	if strings.TrimSpace(m.ImplementationRevision) == "" {
		return fmt.Errorf("portfolio: implementation revision is required")
	}
	if err := m.SourceExperiment.Validate(); err != nil {
		return err
	}
	if err := m.SourceManifestHash.Validate(); err != nil {
		return err
	}
	if err := m.Range.Validate(); err != nil {
		return err
	}
	if !m.Range.Start.Before(m.Range.End) {
		return fmt.Errorf("portfolio: evaluation range must be non-empty")
	}
	if m.Universe.Exchange == "" || !m.Universe.Spot || len(m.Universe.Symbols) < 2 {
		return fmt.Errorf("portfolio: spot universe with at least two symbols is required")
	}
	seenSymbols := make(map[protocolv2.Symbol]bool, len(m.Universe.Symbols))
	for _, snapshot := range m.Universe.Symbols {
		if err := snapshot.Symbol.Validate(); err != nil {
			return err
		}
		if err := snapshot.CandleSHA256.Validate(); err != nil {
			return err
		}
		if seenSymbols[snapshot.Symbol] {
			return fmt.Errorf("portfolio: duplicate universe symbol %s", snapshot.Symbol)
		}
		seenSymbols[snapshot.Symbol] = true
	}
	if !m.Diagnostic && len(m.SignalArtifacts) == 0 {
		return fmt.Errorf("portfolio: primary portfolio requires frozen research-pass artifacts")
	}
	for _, ref := range m.SignalArtifacts {
		if ref.Path == "" || ref.SHA256.Validate() != nil || ref.Experiment.Validate() != nil || ref.Strategy.Validate() != nil || ref.Candidate.Validate() != nil {
			return fmt.Errorf("portfolio: invalid signal artifact reference")
		}
		if !m.Diagnostic && ref.Decision != "research-pass" {
			return fmt.Errorf("portfolio: primary portfolio rejects %s input", ref.Decision)
		}
	}
	if err := validateCosts(m.BaseCosts); err != nil {
		return err
	}
	if err := validateCosts(m.StressCosts); err != nil {
		return err
	}
	l := m.Limits
	if !positive(l.InitialCapital) || !percent(l.RiskPerTradePercent) || !percent(l.MaxPositionPercent) || l.MaxPositions < 1 || !percent(l.MaxAggregateRiskPct) {
		return fmt.Errorf("portfolio: invalid limits")
	}
	r := m.RelativeStrength
	if r.ReturnLookbackDays < 2 || r.VolatilityDays < 2 || r.ATRDays < 2 || !positive(r.StopATR) || r.TopK < 1 || r.ExitRank < r.TopK || r.BTCEMADays < 2 || r.MinPositiveBreadth < 0 || r.MinPositiveBreadth > 1 || r.RebalanceWeekday < time.Sunday || r.RebalanceWeekday > time.Saturday || r.RegimeMode.Validate() != nil || r.EntryMode.Validate() != nil {
		return fmt.Errorf("portfolio: invalid relative-strength config")
	}
	if r.EntryMode.normalized() == EntryModeTrendPullback && (r.PullbackEMADays < 2 || !positive(r.MaxEntryDistanceATR)) {
		return fmt.Errorf("portfolio: invalid pullback entry config")
	}
	if !finite(m.Gates.MinNetReturn) || !finite(m.Gates.MaxDrawdown) || m.Gates.MaxDrawdown <= 0 || !finite(m.Gates.MaxContribution) || m.Gates.MaxContribution <= 0 || m.Gates.MaxContribution > 1 {
		return fmt.Errorf("portfolio: invalid gates")
	}
	if m.ID != "" {
		if err := m.ID.Validate(); err != nil {
			return err
		}
		if err := m.Hash.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (m Manifest) VerifyIdentity() error {
	expected, err := m.payloadHash()
	if err != nil {
		return err
	}
	if expected != m.Hash || m.ID != protocolv2.ExperimentID("portfolio-"+string(expected[:24])) {
		return fmt.Errorf("portfolio: manifest identity mismatch")
	}
	return nil
}

func (m Manifest) payloadHash() (protocolv2.SHA256Hex, error) {
	copy := m
	copy.ID, copy.Hash = "", ""
	copy.SignalArtifacts = append([]SignalArtifactRef(nil), copy.SignalArtifacts...)
	copy.Universe.Symbols = append([]manifest.SymbolSnapshot(nil), copy.Universe.Symbols...)
	sort.Slice(copy.SignalArtifacts, func(i, j int) bool { return copy.SignalArtifacts[i].Path < copy.SignalArtifacts[j].Path })
	sort.Slice(copy.Universe.Symbols, func(i, j int) bool { return copy.Universe.Symbols[i].Symbol < copy.Universe.Symbols[j].Symbol })
	raw, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return protocolv2.SHA256Hex(hex.EncodeToString(sum[:])), nil
}

func DecodeManifest(raw []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("portfolio: decode manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return m, err
	}
	if err := m.VerifyIdentity(); err != nil {
		return m, err
	}
	return m, nil
}

func (m Manifest) Marshal() ([]byte, error) { return json.MarshalIndent(m, "", "  ") }

func VerifySignalArtifacts(refs []SignalArtifactRef) error {
	for _, ref := range refs {
		raw, err := os.ReadFile(ref.Path)
		if err != nil {
			return fmt.Errorf("portfolio: read signal artifact: %w", err)
		}
		sum := sha256.Sum256(raw)
		if protocolv2.SHA256Hex(hex.EncodeToString(sum[:])) != ref.SHA256 {
			return fmt.Errorf("portfolio: signal artifact checksum mismatch: %s", ref.Path)
		}
	}
	return nil
}

func validateCosts(c CostProfile) error {
	if !finite(c.CommissionBPS) || c.CommissionBPS < 0 || !finite(c.SlippageBPS) || c.SlippageBPS < 0 {
		return fmt.Errorf("portfolio: invalid costs")
	}
	return nil
}
func finite(v float64) bool   { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func positive(v float64) bool { return finite(v) && v > 0 }
func percent(v float64) bool  { return positive(v) && v <= 100 }

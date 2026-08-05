package reporting

import (
	"fmt"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/eligibility"
	"github.com/drybin/fear-and-greed/internal/research/execution"
	"github.com/drybin/fear-and-greed/internal/research/metrics"
	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

const (
	SummarySchemaVersion     = "report.summary.v1"
	FoldSchemaVersion        = "report.fold.v1"
	CandidateSchemaVersion   = "report.candidate.v1"
	TradeSchemaVersion       = "report.trade.v1"
	FillSchemaVersion        = "report.fill.v1"
	EquitySchemaVersion      = "report.equity.v1"
	RejectionSchemaVersion   = "report.rejection.v1"
	EligibilitySchemaVersion = "report.eligibility.v1"
)

// Header supplies the mandatory protocol-v2 artifact header.
func Header(schemaVersion string) protocolv2.ArtifactHeader {
	return protocolv2.ArtifactHeader{SchemaVersion: schemaVersion, ProtocolVersion: protocolv2.ReportSchemaVersion}
}

// SummaryReport is the final aggregate evidence for one experiment.
type SummaryReport struct {
	protocolv2.ArtifactHeader
	ExperimentID    protocolv2.ExperimentID `json:"experiment_id"`
	GeneratedAt     time.Time               `json:"generated_at"`
	Metrics         metrics.Summary         `json:"metrics"`
	FoldConsistency *float64                `json:"fold_consistency,omitempty"`
}

// FoldReport retains all measured evidence for one candidate in one fold.
type FoldReport struct {
	protocolv2.ArtifactHeader
	ExperimentID protocolv2.ExperimentID         `json:"experiment_id"`
	FoldID       protocolv2.FoldID               `json:"fold_id"`
	CandidateID  protocolv2.ParameterCandidateID `json:"candidate_id"`
	Strategy     protocolv2.StrategyRef          `json:"strategy"`
	Range        protocolv2.TimeRange            `json:"range"`
	Metrics      metrics.Summary                 `json:"metrics"`
}

// SelectionExplanation records a deterministic selection decision without
// implementing selection or gates. The scorer/gate owner supplies its inputs.
type SelectionExplanation struct {
	Rank         int      `json:"rank"`
	Score        float64  `json:"score"`
	TieBreakKeys []string `json:"tie_break_keys"`
	Selected     bool     `json:"selected"`
	Explanation  string   `json:"explanation"`
}

// CandidateReport retains every parameter candidate, including unselected ones.
type CandidateReport struct {
	protocolv2.ArtifactHeader
	ExperimentID protocolv2.ExperimentID         `json:"experiment_id"`
	CandidateID  protocolv2.ParameterCandidateID `json:"candidate_id"`
	Strategy     protocolv2.StrategyRef          `json:"strategy"`
	Folds        []FoldReport                    `json:"folds"`
	Selection    SelectionExplanation            `json:"selection"`
}

type TradeReport struct {
	protocolv2.ArtifactHeader
	ExperimentID protocolv2.ExperimentID `json:"experiment_id"`
	FoldID       protocolv2.FoldID       `json:"fold_id"`
	Trades       []execution.TradeState  `json:"trades"`
}

// FillRecord is a tagged union over the execution fill evidence.
type FillRecord struct {
	Kind        string                     `json:"kind"`
	Entry       *execution.EntryFill       `json:"entry,omitempty"`
	PartialExit *execution.PartialExitFill `json:"partial_exit,omitempty"`
	FinalExit   *execution.FinalExitFill   `json:"final_exit,omitempty"`
}

type FillReport struct {
	protocolv2.ArtifactHeader
	ExperimentID protocolv2.ExperimentID `json:"experiment_id"`
	FoldID       protocolv2.FoldID       `json:"fold_id"`
	Fills        []FillRecord            `json:"fills"`
}

type EquityReport struct {
	protocolv2.ArtifactHeader
	ExperimentID protocolv2.ExperimentID    `json:"experiment_id"`
	FoldID       protocolv2.FoldID          `json:"fold_id"`
	Equity       []execution.EquitySnapshot `json:"equity"`
}

type RejectionReport struct {
	protocolv2.ArtifactHeader
	ExperimentID protocolv2.ExperimentID     `json:"experiment_id"`
	FoldID       protocolv2.FoldID           `json:"fold_id"`
	Rejections   []execution.SignalRejection `json:"rejections"`
}

type EligibilityReport struct {
	protocolv2.ArtifactHeader
	ExperimentID protocolv2.ExperimentID  `json:"experiment_id"`
	Cohort       eligibility.CohortReport `json:"cohort"`
}

// Validate checks schema identity and shallow invariants before a report is
// written. Execution records validate their own detailed invariants.
func Validate(v any) error {
	switch r := v.(type) {
	case SummaryReport:
		return validateHeader(r.ArtifactHeader, SummarySchemaVersion)
	case FoldReport:
		return validateFold(r)
	case CandidateReport:
		return validateCandidate(r)
	case TradeReport:
		return validateHeader(r.ArtifactHeader, TradeSchemaVersion)
	case FillReport:
		return validateFills(r)
	case EquityReport:
		return validateHeader(r.ArtifactHeader, EquitySchemaVersion)
	case RejectionReport:
		return validateHeader(r.ArtifactHeader, RejectionSchemaVersion)
	case EligibilityReport:
		return validateHeader(r.ArtifactHeader, EligibilitySchemaVersion)
	default:
		return fmt.Errorf("reporting: unsupported artifact type %T", v)
	}
}

func validateHeader(h protocolv2.ArtifactHeader, schema string) error {
	if h.SchemaVersion != schema || h.ProtocolVersion != protocolv2.ReportSchemaVersion {
		return fmt.Errorf("reporting: expected %s/%s header", schema, protocolv2.ReportSchemaVersion)
	}
	return nil
}
func validateFold(r FoldReport) error {
	if err := validateHeader(r.ArtifactHeader, FoldSchemaVersion); err != nil {
		return err
	}
	if err := r.FoldID.Validate(); err != nil {
		return err
	}
	if err := r.CandidateID.Validate(); err != nil {
		return err
	}
	return r.Range.Validate()
}
func validateCandidate(r CandidateReport) error {
	if err := validateHeader(r.ArtifactHeader, CandidateSchemaVersion); err != nil {
		return err
	}
	if err := r.CandidateID.Validate(); err != nil {
		return err
	}
	if r.Selection.Rank < 1 || r.Selection.Explanation == "" {
		return fmt.Errorf("reporting: candidate selection rank and explanation are required")
	}
	for _, fold := range r.Folds {
		if err := validateFold(fold); err != nil {
			return err
		}
	}
	return nil
}
func validateFills(r FillReport) error {
	if err := validateHeader(r.ArtifactHeader, FillSchemaVersion); err != nil {
		return err
	}
	for _, f := range r.Fills {
		switch f.Kind {
		case "entry":
			if f.Entry == nil || f.PartialExit != nil || f.FinalExit != nil {
				return fmt.Errorf("reporting: invalid entry fill record")
			}
		case "partial_exit":
			if f.PartialExit == nil || f.Entry != nil || f.FinalExit != nil {
				return fmt.Errorf("reporting: invalid partial-exit fill record")
			}
		case "final_exit":
			if f.FinalExit == nil || f.Entry != nil || f.PartialExit != nil {
				return fmt.Errorf("reporting: invalid final-exit fill record")
			}
		default:
			return fmt.Errorf("reporting: invalid fill record kind %q", f.Kind)
		}
	}
	return nil
}

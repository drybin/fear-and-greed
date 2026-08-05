package protocolv2

import "fmt"

// Phase is a top-level research workflow phase.
type Phase string

const (
	PhaseVerify      Phase = "verify"
	PhaseDevelopment Phase = "development"
	PhaseFreeze      Phase = "freeze"
	PhaseFinal       Phase = "final"
)

func (p Phase) Validate() error {
	switch p {
	case PhaseVerify, PhaseDevelopment, PhaseFreeze, PhaseFinal:
		return nil
	default:
		return fmt.Errorf("protocolv2: invalid phase %q", p)
	}
}

// RunStatus is the lifecycle status of a phase run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusInterrupted RunStatus = "interrupted"
)

func (s RunStatus) Validate() error {
	switch s {
	case RunStatusPending, RunStatusRunning, RunStatusCompleted, RunStatusFailed, RunStatusInterrupted:
		return nil
	default:
		return fmt.Errorf("protocolv2: invalid run status %q", s)
	}
}

// CohortStatus classifies symbol membership in primary vs secondary cohorts.
type CohortStatus string

const (
	CohortPrimary   CohortStatus = "primary"
	CohortSecondary CohortStatus = "secondary"
	CohortExcluded  CohortStatus = "excluded"
)

func (s CohortStatus) Validate() error {
	switch s {
	case CohortPrimary, CohortSecondary, CohortExcluded:
		return nil
	default:
		return fmt.Errorf("protocolv2: invalid cohort status %q", s)
	}
}

// UniverseProvenance describes how a symbol universe was constructed.
type UniverseProvenance string

const (
	UniverseFrozenCurrentCohort UniverseProvenance = "frozen-current-cohort"
	UniversePointInTime         UniverseProvenance = "point-in-time-universe"
)

func (p UniverseProvenance) Validate() error {
	switch p {
	case UniverseFrozenCurrentCohort, UniversePointInTime:
		return nil
	default:
		return fmt.Errorf("protocolv2: invalid universe provenance %q", p)
	}
}

// DecisionStatus is a standalone research gate outcome.
// It is not a live-trading or portfolio promotion decision.
type DecisionStatus string

const (
	DecisionResearchPass DecisionStatus = "research-pass"
	DecisionObserve      DecisionStatus = "observe"
	DecisionReject       DecisionStatus = "reject"
	DecisionExploratory  DecisionStatus = "exploratory"
)

func (s DecisionStatus) Validate() error {
	switch s {
	case DecisionResearchPass, DecisionObserve, DecisionReject, DecisionExploratory:
		return nil
	default:
		return fmt.Errorf("protocolv2: invalid decision status %q", s)
	}
}

// RejectionReason is a structured reason a signal or unit was rejected.
type RejectionReason string

const (
	RejectionInvalidStop          RejectionReason = "invalid_stop"
	RejectionInsufficientCash     RejectionReason = "insufficient_cash"
	RejectionMissingNextBar       RejectionReason = "missing_next_bar"
	RejectionInsufficientWarmup   RejectionReason = "insufficient_warmup"
	RejectionShortHistory         RejectionReason = "short_history"
	RejectionDataQuality          RejectionReason = "data_quality"
	RejectionHoldoutAccessDenied  RejectionReason = "holdout_access_denied"
	RejectionDuplicateRegistration RejectionReason = "duplicate_registration"
	RejectionInvalidQuantity      RejectionReason = "invalid_quantity"
	RejectionNotionalCap          RejectionReason = "notional_cap"
	RejectionUnknownStrategy      RejectionReason = "unknown_strategy"
	RejectionDeferredStrategy     RejectionReason = "deferred_strategy"
)

func (r RejectionReason) Validate() error {
	switch r {
	case RejectionInvalidStop, RejectionInsufficientCash, RejectionMissingNextBar,
		RejectionInsufficientWarmup, RejectionShortHistory, RejectionDataQuality,
		RejectionHoldoutAccessDenied, RejectionDuplicateRegistration,
		RejectionInvalidQuantity, RejectionNotionalCap, RejectionUnknownStrategy,
		RejectionDeferredStrategy:
		return nil
	default:
		return fmt.Errorf("protocolv2: invalid rejection reason %q", r)
	}
}

// ExperimentClass labels whether a run can produce research-pass.
type ExperimentClass string

const (
	ExperimentPromotable  ExperimentClass = "promotable"
	ExperimentExploratory ExperimentClass = "exploratory"
)

func (c ExperimentClass) Validate() error {
	switch c {
	case ExperimentPromotable, ExperimentExploratory:
		return nil
	default:
		return fmt.Errorf("protocolv2: invalid experiment class %q", c)
	}
}

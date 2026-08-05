package protocolv2

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reExperimentID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
	reStrategyCode = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	reVersion      = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)
	reSymbol       = regexp.MustCompile(`^[A-Z0-9]{2,32}$`)
	reHex64        = regexp.MustCompile(`^[a-f0-9]{64}$`)
)

// ExperimentID is a stable run identity string (lowercase slug).
type ExperimentID string

func (id ExperimentID) Validate() error {
	s := string(id)
	if !reExperimentID.MatchString(s) {
		return fmt.Errorf("protocolv2: invalid experiment id %q", s)
	}
	return nil
}

// StrategyCode is a strategy registry code, e.g. fib-pullback-trend-v1.
type StrategyCode string

func (c StrategyCode) Validate() error {
	s := string(c)
	if !reStrategyCode.MatchString(s) {
		return fmt.Errorf("protocolv2: invalid strategy code %q", s)
	}
	return nil
}

// StrategyVersion identifies a behavioral strategy revision.
type StrategyVersion string

func (v StrategyVersion) Validate() error {
	s := string(v)
	if !reVersion.MatchString(s) {
		return fmt.Errorf("protocolv2: invalid strategy version %q", s)
	}
	return nil
}

// StrategyRef binds a code to a version.
type StrategyRef struct {
	Code    StrategyCode    `json:"code"`
	Version StrategyVersion `json:"version"`
}

func (r StrategyRef) String() string {
	return string(r.Code) + "@" + string(r.Version)
}

func (r StrategyRef) Validate() error {
	if err := r.Code.Validate(); err != nil {
		return err
	}
	return r.Version.Validate()
}

// ParameterCandidateID is a stable id for one grid point inside a strategy.
type ParameterCandidateID string

func (id ParameterCandidateID) Validate() error {
	s := string(id)
	if s == "" || len(s) > 128 {
		return fmt.Errorf("protocolv2: invalid parameter candidate id %q", s)
	}
	return nil
}

// FoldID identifies a walk-forward fold within an experiment.
type FoldID string

func (id FoldID) Validate() error {
	s := string(id)
	if s == "" || len(s) > 64 {
		return fmt.Errorf("protocolv2: invalid fold id %q", s)
	}
	return nil
}

// Symbol is a spot market pair base+quote without separator, e.g. BTCUSDT.
type Symbol string

func (s Symbol) Validate() error {
	v := string(s)
	if !reSymbol.MatchString(v) {
		return fmt.Errorf("protocolv2: invalid symbol %q", v)
	}
	return nil
}

// Timeframe is a bar interval label, e.g. 1m, 1h, 4h.
type Timeframe string

func (tf Timeframe) Validate() error {
	switch tf {
	case "1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "1d":
		return nil
	default:
		return fmt.Errorf("protocolv2: invalid timeframe %q", tf)
	}
}

// CostProfileID names a frozen cost profile in the manifest.
type CostProfileID string

func (id CostProfileID) Validate() error {
	s := strings.TrimSpace(string(id))
	if s == "" || len(s) > 64 {
		return fmt.Errorf("protocolv2: invalid cost profile id %q", id)
	}
	return nil
}

// SizingProfileID names a frozen sizing profile in the manifest.
type SizingProfileID string

func (id SizingProfileID) Validate() error {
	s := strings.TrimSpace(string(id))
	if s == "" || len(s) > 64 {
		return fmt.Errorf("protocolv2: invalid sizing profile id %q", id)
	}
	return nil
}

// SHA256Hex is a lowercase hex-encoded SHA-256 digest.
type SHA256Hex string

func (h SHA256Hex) Validate() error {
	if !reHex64.MatchString(string(h)) {
		return fmt.Errorf("protocolv2: invalid sha256 hex digest")
	}
	return nil
}

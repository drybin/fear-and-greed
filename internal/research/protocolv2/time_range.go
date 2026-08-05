package protocolv2

import (
	"fmt"
	"time"
)

// TimeRange is a half-open UTC interval [Start, End).
// Start is inclusive; End is exclusive. Both must be UTC with zero location
// offset (time.UTC). Instantaneous empty ranges (Start == End) are allowed;
// inverted ranges (End.Before(Start)) are invalid.
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

func NewTimeRange(start, end time.Time) (TimeRange, error) {
	r := TimeRange{Start: start.UTC(), End: end.UTC()}
	if err := r.Validate(); err != nil {
		return TimeRange{}, err
	}
	return r, nil
}

func (r TimeRange) Validate() error {
	if r.Start.Location() != time.UTC {
		return fmt.Errorf("protocolv2: time range start must be UTC")
	}
	if r.End.Location() != time.UTC {
		return fmt.Errorf("protocolv2: time range end must be UTC")
	}
	if r.End.Before(r.Start) {
		return fmt.Errorf("protocolv2: time range end %s before start %s", r.End, r.Start)
	}
	return nil
}

// ContainsInstant reports whether t is in [Start, End).
func (r TimeRange) ContainsInstant(t time.Time) bool {
	t = t.UTC()
	return !t.Before(r.Start) && t.Before(r.End)
}

// ContainsRange reports whether other is fully inside this range.
func (r TimeRange) ContainsRange(other TimeRange) bool {
	return !other.Start.Before(r.Start) && !other.End.After(r.End)
}

// Overlaps reports whether the half-open intervals intersect.
func (r TimeRange) Overlaps(other TimeRange) bool {
	return r.Start.Before(other.End) && other.Start.Before(r.End)
}

// Duration returns End - Start.
func (r TimeRange) Duration() time.Duration {
	return r.End.Sub(r.Start)
}

// MustUTC panics if t is not in UTC location; otherwise returns t unchanged.
func MustUTC(t time.Time) time.Time {
	if t.Location() != time.UTC {
		panic("protocolv2: time must be UTC")
	}
	return t
}

// AsUTC normalizes any time to UTC.
func AsUTC(t time.Time) time.Time {
	return t.UTC()
}

package eligibility

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/drybin/fear-and-greed/internal/research/protocolv2"
)

const (
	// SurvivorshipWarning is required on reports based on a current, frozen
	// watchlist. It prevents interpreting the result as historical top-N data.
	SurvivorshipWarning = "Frozen-current-cohort results describe this frozen watchlist tested backward; they do not establish historical top-N performance."
)

type IssueKind string

const (
	IssueMalformed     IssueKind = "malformed"
	IssueNonFiniteOHLC IssueKind = "non-finite-ohlc"
	IssueUnordered     IssueKind = "unordered"
	IssueDuplicate     IssueKind = "duplicate"
	IssueMissing       IssueKind = "missing"
	IssueVolume        IssueKind = "volume"
)

type Issue struct {
	Kind    IssueKind `json:"kind"`
	Row     int       `json:"row,omitempty"`
	Message string    `json:"message"`
}

// VolumeInventory is intentionally separate from core candle quality: none of
// the core strategies requires volume, so a volume fault is reported but does
// not make a candle file ineligible on its own.
type VolumeInventory struct {
	Present       bool `json:"present"`
	MissingRows   int  `json:"missing_rows"`
	MalformedRows int  `json:"malformed_rows"`
	NonFiniteRows int  `json:"non_finite_rows"`
	ZeroRows      int  `json:"zero_rows"`
	PositiveRows  int  `json:"positive_rows"`
}

// Usable reports whether a volume-dependent strategy can trust this source.
func (v VolumeInventory) Usable() bool {
	// A zero-volume minute can be a valid no-trade Binance candle. What makes
	// a source unusable is a missing/invalid value or a fully zero-filled file.
	return v.Present && v.MissingRows == 0 && v.MalformedRows == 0 && v.NonFiniteRows == 0 && v.PositiveRows > 0
}

// FileInventory describes one immutable candle input. SHA256 is calculated
// over the raw file bytes, not parsed CSV values, so any edit to an input is
// reproducibly visible in the manifest.
type FileInventory struct {
	Path       string               `json:"path"`
	SHA256     protocolv2.SHA256Hex `json:"sha256"`
	Columns    []string             `json:"columns"`
	RowCount   int                  `json:"row_count"`
	Range      protocolv2.TimeRange `json:"range"`
	Interval   time.Duration        `json:"interval"`
	Volume     VolumeInventory      `json:"volume"`
	Issues     []Issue              `json:"issues,omitempty"`
	Timestamps []time.Time          `json:"-"`
}

func (i FileInventory) CoreUsable() bool {
	for _, issue := range i.Issues {
		if issue.Kind != IssueVolume {
			return false
		}
	}
	return i.RowCount > 0 && len(i.Timestamps) > 0
}

// InventoryFile parses the fetch-data CSV format (open_time, open, high, low,
// close, volume...). Malformed rows are retained as issues instead of stopping
// at the first fault, allowing the caller to report every data-quality defect.
func InventoryFile(path string) (FileInventory, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return FileInventory{}, fmt.Errorf("eligibility: read candle file: %w", err)
	}
	sum := sha256.Sum256(raw)
	result := FileInventory{
		Path:   path,
		SHA256: protocolv2.SHA256Hex(hex.EncodeToString(sum[:])),
	}

	reader := csv.NewReader(strings.NewReader(string(raw)))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return result, fmt.Errorf("eligibility: read CSV header: %w", err)
	}
	result.Columns = append([]string(nil), header...)
	if len(header) < 5 {
		result.Issues = append(result.Issues, Issue{Kind: IssueMalformed, Row: 1, Message: "expected at least five columns"})
		return result, nil
	}

	var prior time.Time
	for row := 2; ; row++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		result.RowCount++
		if err != nil {
			result.Issues = append(result.Issues, Issue{Kind: IssueMalformed, Row: row, Message: err.Error()})
			continue
		}
		if len(record) < 5 {
			result.Issues = append(result.Issues, Issue{Kind: IssueMalformed, Row: row, Message: "expected at least five columns"})
			continue
		}
		ts, values, parseErr := parseCore(record)
		if parseErr != nil {
			result.Issues = append(result.Issues, Issue{Kind: IssueMalformed, Row: row, Message: parseErr.Error()})
			continue
		}
		if !finite(values[0]) || !finite(values[1]) || !finite(values[2]) || !finite(values[3]) {
			result.Issues = append(result.Issues, Issue{Kind: IssueNonFiniteOHLC, Row: row, Message: "OHLC values must be finite"})
			continue
		}
		if !prior.IsZero() {
			if ts.Before(prior) {
				result.Issues = append(result.Issues, Issue{Kind: IssueUnordered, Row: row, Message: "open_time is before the preceding row"})
			} else if ts.Equal(prior) {
				result.Issues = append(result.Issues, Issue{Kind: IssueDuplicate, Row: row, Message: "duplicate open_time"})
			}
		}
		prior = ts
		result.Timestamps = append(result.Timestamps, ts)
		if len(record) <= 5 || strings.TrimSpace(record[5]) == "" {
			result.Volume.MissingRows++
			result.Issues = append(result.Issues, Issue{Kind: IssueVolume, Row: row, Message: "missing volume"})
		} else {
			result.Volume.Present = true
			volume, volumeErr := parseFloat(record[5])
			if volumeErr != nil {
				result.Volume.MalformedRows++
				result.Issues = append(result.Issues, Issue{Kind: IssueVolume, Row: row, Message: "invalid volume"})
			} else if !finite(volume) {
				result.Volume.NonFiniteRows++
				result.Issues = append(result.Issues, Issue{Kind: IssueVolume, Row: row, Message: "volume must be finite"})
			} else if volume <= 0 {
				result.Volume.ZeroRows++
			} else {
				result.Volume.PositiveRows++
			}
		}
	}
	result.detectRangeIntervalAndGaps()
	return result, nil
}

func parseCore(record []string) (time.Time, [4]float64, error) {
	ts, err := time.ParseInLocation("2006-01-02 15:04:05", record[0], time.UTC)
	if err != nil {
		return time.Time{}, [4]float64{}, fmt.Errorf("invalid open_time")
	}
	var values [4]float64
	for idx := range values {
		values[idx], err = parseFloat(record[idx+1])
		if err != nil {
			return time.Time{}, values, fmt.Errorf("invalid OHLC")
		}
	}
	return ts, values, nil
}

func parseFloat(v string) (float64, error) {
	var parsed float64
	_, err := fmt.Sscan(strings.TrimSpace(v), &parsed)
	return parsed, err
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

func (i *FileInventory) detectRangeIntervalAndGaps() {
	if len(i.Timestamps) == 0 {
		return
	}
	timestamps := append([]time.Time(nil), i.Timestamps...)
	sort.Slice(timestamps, func(a, b int) bool { return timestamps[a].Before(timestamps[b]) })
	i.Range = protocolv2.TimeRange{Start: timestamps[0], End: timestamps[len(timestamps)-1]}
	if len(timestamps) < 2 {
		return
	}
	counts := make(map[time.Duration]int)
	for n := 1; n < len(timestamps); n++ {
		if delta := timestamps[n].Sub(timestamps[n-1]); delta > 0 {
			counts[delta]++
		}
	}
	for interval, count := range counts {
		if count > counts[i.Interval] || (count == counts[i.Interval] && (i.Interval == 0 || interval < i.Interval)) {
			i.Interval = interval
		}
	}
	if i.Interval == 0 {
		return
	}
	i.Range.End = i.Range.End.Add(i.Interval)
	for n := 1; n < len(timestamps); n++ {
		delta := timestamps[n].Sub(timestamps[n-1])
		if delta > i.Interval {
			missing := int(delta/i.Interval) - 1
			if missing > 0 {
				i.Issues = append(i.Issues, Issue{Kind: IssueMissing, Message: fmt.Sprintf("%d missing interval(s) after %s", missing, timestamps[n-1].Format(time.RFC3339))})
			}
		}
	}
}

// FrozenSnapshot is a dated, immutable symbol list. The supplied asOf date is
// explicit because the existing scripts/symbols_*.txt format is intentionally
// one symbol per non-comment line and has no machine-readable date field.
type FrozenSnapshot struct {
	Name       string               `json:"name"`
	AsOf       time.Time            `json:"as_of"`
	SourceFile string               `json:"source_file"`
	SHA256     protocolv2.SHA256Hex `json:"sha256"`
	Symbols    []protocolv2.Symbol  `json:"symbols"`
}

func LoadFrozenSnapshot(path, name string, asOf time.Time) (FrozenSnapshot, error) {
	if name == "" || asOf.IsZero() {
		return FrozenSnapshot{}, fmt.Errorf("eligibility: snapshot name and date are required")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return FrozenSnapshot{}, fmt.Errorf("eligibility: read snapshot: %w", err)
	}
	sum := sha256.Sum256(raw)
	snapshot := FrozenSnapshot{Name: name, AsOf: asOf.UTC(), SourceFile: path, SHA256: protocolv2.SHA256Hex(hex.EncodeToString(sum[:]))}
	seen := make(map[protocolv2.Symbol]bool)
	for lineNo, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		symbol := protocolv2.Symbol(line)
		if err := symbol.Validate(); err != nil {
			return FrozenSnapshot{}, fmt.Errorf("eligibility: snapshot line %d: %w", lineNo+1, err)
		}
		if seen[symbol] {
			return FrozenSnapshot{}, fmt.Errorf("eligibility: snapshot line %d: duplicate symbol %s", lineNo+1, symbol)
		}
		seen[symbol] = true
		snapshot.Symbols = append(snapshot.Symbols, symbol)
	}
	if len(snapshot.Symbols) == 0 {
		return FrozenSnapshot{}, fmt.Errorf("eligibility: snapshot contains no symbols")
	}
	return snapshot, nil
}

type ExclusionReason string

const (
	ExclusionDataQuality        ExclusionReason = "data-quality"
	ExclusionShortHistory       ExclusionReason = "short-history"
	ExclusionInsufficientWarmup ExclusionReason = "insufficient-warmup"
)

type SymbolEligibility struct {
	Symbol           protocolv2.Symbol    `json:"symbol"`
	Primary          bool                 `json:"primary"`
	Secondary        bool                 `json:"secondary"`
	Warmup           protocolv2.TimeRange `json:"warmup"`
	ExclusionReasons []ExclusionReason    `json:"exclusion_reasons,omitempty"`
}

type CohortReport struct {
	Provenance          protocolv2.UniverseProvenance `json:"provenance"`
	SurvivorshipWarning string                        `json:"survivorship_warning"`
	Primary             []SymbolEligibility           `json:"primary"`
	Secondary           []SymbolEligibility           `json:"secondary"`
}

// EvaluateCohort applies core data-quality, 12-month pre-test history, and
// strategy warmup rules. Warmup is only an eligibility range: execution must
// begin PnL accounting at test.Start.
func EvaluateCohort(snapshot FrozenSnapshot, inventories map[protocolv2.Symbol]FileInventory, test protocolv2.TimeRange, warmupBars int) (CohortReport, error) {
	if err := test.Validate(); err != nil || test.Start.Equal(test.End) {
		return CohortReport{}, fmt.Errorf("eligibility: invalid test range")
	}
	if warmupBars < 0 {
		return CohortReport{}, fmt.Errorf("eligibility: warmup bars cannot be negative")
	}
	report := CohortReport{Provenance: protocolv2.UniverseFrozenCurrentCohort, SurvivorshipWarning: SurvivorshipWarning}
	for _, symbol := range snapshot.Symbols {
		entry := SymbolEligibility{Symbol: symbol}
		inventory, found := inventories[symbol]
		if !found || !inventory.CoreUsable() {
			entry.ExclusionReasons = append(entry.ExclusionReasons, ExclusionDataQuality)
		} else {
			if inventory.Range.Start.After(test.Start.AddDate(-1, 0, 0)) || inventory.Range.End.Before(test.Start) {
				entry.ExclusionReasons = append(entry.ExclusionReasons, ExclusionShortHistory)
			}
			if warmupBars > 0 {
				index := sort.Search(len(inventory.Timestamps), func(i int) bool { return !inventory.Timestamps[i].Before(test.Start) })
				if index < warmupBars {
					entry.ExclusionReasons = append(entry.ExclusionReasons, ExclusionInsufficientWarmup)
				} else {
					entry.Warmup = protocolv2.TimeRange{Start: inventory.Timestamps[index-warmupBars], End: test.Start}
				}
			}
		}
		entry.Primary = len(entry.ExclusionReasons) == 0
		entry.Secondary = !entry.Primary
		if entry.Primary {
			report.Primary = append(report.Primary, entry)
		} else {
			report.Secondary = append(report.Secondary, entry)
		}
	}
	return report, nil
}

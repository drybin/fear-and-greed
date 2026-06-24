package scanreport

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/infrastructure/csvdata"
	"github.com/drybin/fear-and-greed/internal/strategy"
)

// DefaultChartIntervalMin is the OHLC bucket size written for HTML charts.
const DefaultChartIntervalMin = 240

// OHLCBar is one aggregated candle for the chart JSON.
type OHLCBar struct {
	T int64   `json:"t"` // unix seconds UTC (bar open)
	O float64 `json:"o"`
	H float64 `json:"h"`
	L float64 `json:"l"`
	C float64 `json:"c"`
}

// OHLCFile is sidecar data for report.html charts.
type OHLCFile struct {
	IntervalMinutes int       `json:"interval_minutes"`
	Symbol          string    `json:"symbol"`
	Period          string    `json:"period"`
	From            string    `json:"from"`
	To              string    `json:"to"`
	Candles         []OHLCBar `json:"candles"`
}

// OHLCFileName returns the sidecar filename for a result.
func OHLCFileName(symbol, period string, intervalMin int) string {
	return fmt.Sprintf("%s__%s__ohlc_%dm.json", sanitizeFilePart(symbol), sanitizeFilePart(period), intervalMin)
}

// OHLCPath returns the full path for a result OHLC sidecar.
func OHLCPath(reportRoot string, r Result, intervalMin int) string {
	return filepath.Join(reportRoot, DataSubdir, r.Algo, OHLCFileName(r.Symbol, r.Period, intervalMin))
}

// IsOHLCFile reports whether name is an OHLC sidecar (not a scan result).
func IsOHLCFile(name string) bool {
	return strings.Contains(name, "__ohlc_") && strings.HasSuffix(name, ".json")
}

// BuildOHLC aggregates minute candles to intervalMin bars for charts.
func BuildOHLC(candles []model.Candle, intervalMin int) []OHLCBar {
	if len(candles) == 0 {
		return nil
	}
	if intervalMin < 1 {
		intervalMin = DefaultChartIntervalMin
	}
	agg := strategy.AggregateMinutes(candles, intervalMin)
	out := make([]OHLCBar, 0, len(agg))
	for _, c := range agg {
		out = append(out, OHLCBar{
			T: c.OpenTime.UTC().Unix(),
			O: c.Open,
			H: c.High,
			L: c.Low,
			C: c.Close,
		})
	}
	return out
}

// SaveOHLC writes aggregated OHLC sidecar next to the result JSON.
func SaveOHLC(reportRoot string, r Result, candles []model.Candle, intervalMin int) error {
	if reportRoot == "" || r.Algo == "" || len(candles) == 0 {
		return nil
	}
	if intervalMin < 1 {
		intervalMin = DefaultChartIntervalMin
	}
	path := OHLCPath(reportRoot, r, intervalMin)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body := OHLCFile{
		IntervalMinutes: intervalMin,
		Symbol:          r.Symbol,
		Period:          r.Period,
		From:            r.CandleFrom,
		To:              r.CandleTo,
		Candles:         BuildOHLC(candles, intervalMin),
	}
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// EnsureOHLC writes missing OHLC sidecars by reading raw CSV from dataDir.
// Individual failures are skipped so HTML generation can continue.
func EnsureOHLC(reportRoot, dataDir string, results []Result, intervalMin int) {
	if intervalMin < 1 {
		intervalMin = DefaultChartIntervalMin
	}
	for _, r := range results {
		if r.Algo == "" || r.Symbol == "" || r.Period == "" {
			continue
		}
		path := OHLCPath(reportRoot, r, intervalMin)
		if _, err := os.Stat(path); err == nil {
			continue
		}
		_ = backfillOHLC(reportRoot, dataDir, r, intervalMin)
	}
}

func backfillOHLC(reportRoot, dataDir string, r Result, intervalMin int) error {
	csvPath := filepath.Join(dataDir, r.Symbol+".csv")
	candles, err := csvdata.LoadKlines(csvPath)
	if err != nil {
		return err
	}
	from, to, err := parseResultRange(r)
	if err != nil {
		return err
	}
	subset := filterCandles(candles, from, to)
	if len(subset) == 0 {
		return fmt.Errorf("no candles in range %s — %s", r.CandleFrom, r.CandleTo)
	}
	return SaveOHLC(reportRoot, r, subset, intervalMin)
}

func parseResultRange(r Result) (time.Time, time.Time, error) {
	from, err := time.Parse(time.RFC3339, r.CandleFrom)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("candle_from: %w", err)
	}
	to, err := time.Parse(time.RFC3339, r.CandleTo)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("candle_to: %w", err)
	}
	return from.UTC(), to.UTC(), nil
}

func filterCandles(candles []model.Candle, from, to time.Time) []model.Candle {
	out := make([]model.Candle, 0)
	for _, c := range candles {
		if c.OpenTime.Before(from) {
			continue
		}
		if c.OpenTime.After(to) {
			break
		}
		out = append(out, c)
	}
	return out
}

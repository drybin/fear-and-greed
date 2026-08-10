package csvdata

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/pkg/wrap"
)

const timeLayout = "2006-01-02 15:04:05"

// LoadKlines reads fetch-data CSV (open_time, open, high, low, close, ...).
func LoadKlines(path string) ([]model.Candle, error) {
	out, err := loadKlines(path, time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, wrap.Errorf("csv is empty: %s", path)
	}
	return out, nil
}

// LoadKlinesRange reads only candles in the half-open [start, end) range.
// Source files are chronological, so parsing stops as soon as end is reached.
func LoadKlinesRange(path string, start, end time.Time) ([]model.Candle, error) {
	if start.IsZero() || end.IsZero() || !start.Before(end) {
		return nil, wrap.Errorf("invalid candle range")
	}
	return loadKlines(path, start.UTC(), end.UTC())
}

func loadKlines(path string, start, end time.Time) ([]model.Candle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, wrap.Errorf("open csv: %w", err)
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	if _, err := r.Read(); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, wrap.Errorf("csv is empty: %s", path)
		}
		return nil, wrap.Errorf("read csv: %w", err)
	}

	// Read rows incrementally. ReadAll keeps both every CSV field string and
	// every parsed candle alive at once, which is prohibitive for multi-year
	// minute data.
	capacity := 100_000
	if !start.IsZero() {
		capacity = int(end.Sub(start)/time.Minute) + 1
	}
	out := make([]model.Candle, 0, capacity)
	for row := 2; ; row++ {
		rec, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, wrap.Errorf("row %d: read csv: %w", row, err)
		}
		if len(rec) < 5 {
			return nil, wrap.Errorf("row %d: expected at least 5 columns", row)
		}
		ts, err := time.ParseInLocation(timeLayout, rec[0], time.UTC)
		if err != nil {
			return nil, wrap.Errorf("row %d time: %w", row, err)
		}
		if !start.IsZero() {
			if ts.Before(start) {
				continue
			}
			if !ts.Before(end) {
				break
			}
		}
		open, err := strconv.ParseFloat(rec[1], 64)
		if err != nil {
			return nil, wrap.Errorf("row %d open: %w", row, err)
		}
		high, err := strconv.ParseFloat(rec[2], 64)
		if err != nil {
			return nil, wrap.Errorf("row %d high: %w", row, err)
		}
		low, err := strconv.ParseFloat(rec[3], 64)
		if err != nil {
			return nil, wrap.Errorf("row %d low: %w", row, err)
		}
		closePrice, err := strconv.ParseFloat(rec[4], 64)
		if err != nil {
			return nil, wrap.Errorf("row %d close: %w", row, err)
		}
		vol := 0.0
		if len(rec) > 5 {
			vol, err = strconv.ParseFloat(rec[5], 64)
			if err != nil {
				return nil, wrap.Errorf("row %d volume: %w", row, err)
			}
		}
		out = append(out, model.Candle{
			OpenTime: ts,
			Open:     open,
			High:     high,
			Low:      low,
			Close:    closePrice,
			Volume:   vol,
		})
	}
	return out, nil
}

// SymbolFromFilename returns market id from e.g. BTCUSDT.csv or ETHUSDT_futures.csv.
func SymbolFromFilename(name string) string {
	base := name
	if len(base) > 4 && base[len(base)-4:] == ".csv" {
		base = base[:len(base)-4]
	}
	return base
}

// ListCSVFiles returns non-temp .csv paths in dir.
func ListCSVFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, wrap.Errorf("read dir %s: %w", dir, err)
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < 4 || name[len(name)-4:] != ".csv" {
			continue
		}
		if len(name) > 8 && name[len(name)-8:] == ".csv.tmp" {
			continue
		}
		if len(name) > 4 && name[len(name)-4:] == ".tmp" {
			continue
		}
		paths = append(paths, fmt.Sprintf("%s/%s", dir, name))
	}
	return paths, nil
}

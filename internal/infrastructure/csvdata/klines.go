package csvdata

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/pkg/wrap"
)

const timeLayout = "2006-01-02 15:04:05"

// LoadKlines reads fetch-data CSV (open_time, open, high, low, close, ...).
func LoadKlines(path string) ([]model.Candle, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, wrap.Errorf("open csv: %w", err)
	}
	defer func() { _ = f.Close() }()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, wrap.Errorf("read csv: %w", err)
	}
	if len(records) < 2 {
		return nil, wrap.Errorf("csv is empty: %s", path)
	}

	out := make([]model.Candle, 0, len(records)-1)
	for i, rec := range records[1:] {
		if len(rec) < 5 {
			return nil, wrap.Errorf("row %d: expected at least 5 columns", i+2)
		}
		ts, err := time.ParseInLocation(timeLayout, rec[0], time.UTC)
		if err != nil {
			return nil, wrap.Errorf("row %d time: %w", i+2, err)
		}
		open, err := strconv.ParseFloat(rec[1], 64)
		if err != nil {
			return nil, wrap.Errorf("row %d open: %w", i+2, err)
		}
		high, err := strconv.ParseFloat(rec[2], 64)
		if err != nil {
			return nil, wrap.Errorf("row %d high: %w", i+2, err)
		}
		low, err := strconv.ParseFloat(rec[3], 64)
		if err != nil {
			return nil, wrap.Errorf("row %d low: %w", i+2, err)
		}
		closePrice, err := strconv.ParseFloat(rec[4], 64)
		if err != nil {
			return nil, wrap.Errorf("row %d close: %w", i+2, err)
		}
		vol := 0.0
		if len(rec) > 5 {
			vol, err = strconv.ParseFloat(rec[5], 64)
			if err != nil {
				return nil, wrap.Errorf("row %d volume: %w", i+2, err)
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

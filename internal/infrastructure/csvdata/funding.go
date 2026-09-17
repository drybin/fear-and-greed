package csvdata

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

// LoadFundingRates maps UTC settlement timestamps to their settled rate.
func LoadFundingRates(path string) (map[time.Time]float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	if len(header) != 2 || header[0] != "funding_time" || header[1] != "funding_rate" {
		return nil, fmt.Errorf("invalid funding CSV header")
	}
	out := map[time.Time]float64{}
	for row := 2; ; row++ {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(rec) != 2 {
			return nil, fmt.Errorf("funding row %d has %d fields", row, len(rec))
		}
		at, err := time.ParseInLocation(timeLayout, rec[0], time.UTC)
		if err != nil {
			return nil, err
		}
		rate, err := strconv.ParseFloat(rec[1], 64)
		if err != nil {
			return nil, err
		}
		out[at] = rate
	}
	return out, nil
}

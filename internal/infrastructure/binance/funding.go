package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/drybin/fear-and-greed/pkg/wrap"
)

const futuresFundingURL = "https://fapi.binance.com/fapi/v1/fundingRate"

// FundingRate is one settled Binance USD-M funding event. A positive rate is
// paid by longs and received by shorts.
type FundingRate struct {
	Time time.Time
	Rate float64
}

// FundingRates downloads every settled funding event in [start, end).
func (c *Client) FundingRates(ctx context.Context, symbol string, start, end time.Time) ([]FundingRate, error) {
	if !start.Before(end) {
		return nil, fmt.Errorf("funding start must be before end")
	}
	var out []FundingRate
	cursor := start.UTC()
	for cursor.Before(end) {
		q := url.Values{}
		q.Set("symbol", symbol)
		q.Set("startTime", strconv.FormatInt(cursor.UnixMilli(), 10))
		q.Set("endTime", strconv.FormatInt(end.UTC().UnixMilli()-1, 10))
		q.Set("limit", "1000")
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, futuresFundingURL+"?"+q.Encode(), nil)
		if err != nil {
			return nil, wrap.Errorf("build funding request: %w", err)
		}
		req.Header.Set("User-Agent", "fear-and-greed/1.0")
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, wrap.Errorf("binance funding request: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if readErr != nil {
			return nil, wrap.Errorf("read funding response: %w", readErr)
		}
		if closeErr != nil {
			return nil, wrap.Errorf("close funding response: %w", closeErr)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, wrap.Errorf("binance funding status %d: %s", resp.StatusCode, string(body))
		}
		var raw []struct {
			FundingTime int64  `json:"fundingTime"`
			FundingRate string `json:"fundingRate"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return nil, wrap.Errorf("decode funding response: %w", err)
		}
		if len(raw) == 0 {
			break
		}
		for _, row := range raw {
			rate, err := strconv.ParseFloat(row.FundingRate, 64)
			if err != nil {
				return nil, wrap.Errorf("parse funding rate: %w", err)
			}
			at := time.UnixMilli(row.FundingTime).UTC()
			if !at.Before(end) {
				continue
			}
			out = append(out, FundingRate{Time: at, Rate: rate})
		}
		next := time.UnixMilli(raw[len(raw)-1].FundingTime + 1).UTC()
		if !next.After(cursor) {
			break
		}
		cursor = next
		if len(raw) < 1000 {
			break
		}
	}
	return out, nil
}

// FundingCSVFilename is kept distinct from kline data so candles retain the
// exact exchange schema while funding can be fingerprinted independently.
func FundingCSVFilename(symbol string) string { return symbol + "_futures_funding.csv" }

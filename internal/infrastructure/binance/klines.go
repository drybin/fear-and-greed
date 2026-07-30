package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/drybin/fear-and-greed/pkg/wrap"
)

const (
	defaultHTTPTimeout = 120 * time.Second
	defaultRequestGap  = 200 * time.Millisecond
	maxRetries         = 5
)

// Market is spot or USD-M perpetual futures.
type Market string

const (
	MarketSpot    Market = "spot"
	MarketFutures Market = "futures"
)

// Kline is one Binance candle (spot or USD-M futures).
type Kline struct {
	OpenTime            time.Time
	Open                float64
	High                float64
	Low                 float64
	Close               float64
	Volume              float64
	CloseTime           time.Time
	QuoteVolume         float64
	Trades              int64
	TakerBuyBaseVolume  float64
	TakerBuyQuoteVolume float64
}

type Client struct {
	httpClient *http.Client
	requestGap time.Duration
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: defaultHTTPTimeout},
		requestGap: defaultRequestGap,
	}
}

func (m Market) klinesURL() (string, error) {
	switch m {
	case MarketSpot:
		return "https://api.binance.com/api/v3/klines", nil
	case MarketFutures:
		return "https://fapi.binance.com/fapi/v1/klines", nil
	default:
		return "", fmt.Errorf("unknown market %q (use spot or futures)", m)
	}
}

// ParseMarket normalizes CLI value.
func ParseMarket(s string) (Market, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "spot", "":
		return MarketSpot, nil
	case "futures", "future", "fapi":
		return MarketFutures, nil
	default:
		return "", fmt.Errorf("unknown market %q (use spot or futures)", s)
	}
}

// FetchProgress is called after each batch; fetched is cumulative candle count.
type FetchProgress func(fetched, estimatedTotal int64)

// EstimatedCandles approximates how many candles fit in [start, end) for the interval.
func EstimatedCandles(start, end time.Time, interval string) int64 {
	if !start.Before(end) {
		return 0
	}
	step := intervalDuration(interval)
	if step <= 0 {
		return 0
	}
	n := end.Sub(start) / step
	if n < 1 {
		return 1
	}
	return int64(n)
}

// KlineBatchHandler is called for each downloaded batch. Do not retain the slice after return.
type KlineBatchHandler func(batch []Kline) error

// StreamKlines downloads candles for [start, end) and passes each batch to handle.
func (c *Client) StreamKlines(
	ctx context.Context,
	market Market,
	symbol, interval string,
	start, end time.Time,
	handle KlineBatchHandler,
	onProgress FetchProgress,
) (int64, error) {
	if !start.Before(end) {
		return 0, wrap.Errorf("start must be before end")
	}
	if handle == nil {
		return 0, wrap.Errorf("batch handler is required")
	}

	klinesURL, err := market.klinesURL()
	if err != nil {
		return 0, err
	}

	var fetched int64
	cursor := start
	estimated := EstimatedCandles(start, end, interval)

	for cursor.Before(end) {
		batch, err := c.fetchBatchWithRetry(ctx, klinesURL, symbol, interval, cursor, end, 1000)
		if err != nil {
			return fetched, err
		}
		if len(batch) == 0 {
			break
		}

		if err := handle(batch); err != nil {
			return fetched, err
		}

		fetched += int64(len(batch))
		if onProgress != nil {
			onProgress(fetched, estimated)
		}

		last := batch[len(batch)-1]
		next := last.OpenTime.Add(intervalDuration(interval))
		if !next.After(cursor) {
			break
		}
		cursor = next

		select {
		case <-ctx.Done():
			return fetched, ctx.Err()
		case <-time.After(c.requestGap):
		}
	}

	return fetched, nil
}

func (c *Client) fetchBatchWithRetry(
	ctx context.Context,
	klinesURL, symbol, interval string,
	start, end time.Time,
	limit int,
) ([]Kline, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		batch, err := c.fetchBatch(ctx, klinesURL, symbol, interval, start, end, limit)
		if err == nil {
			return batch, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
	}
	return nil, wrap.Errorf("after %d attempts: %w", maxRetries, lastErr)
}

func isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "status 429") ||
		strings.Contains(msg, "status 5") ||
		strings.Contains(msg, "Timeout") ||
		strings.Contains(msg, "connection reset")
}

func (c *Client) fetchBatch(
	ctx context.Context,
	klinesURL, symbol, interval string,
	start, end time.Time,
	limit int,
) ([]Kline, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("interval", interval)
	q.Set("startTime", strconv.FormatInt(start.UnixMilli(), 10))
	q.Set("endTime", strconv.FormatInt(end.UnixMilli(), 10))
	q.Set("limit", strconv.Itoa(limit))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, klinesURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, wrap.Errorf("build klines request: %w", err)
	}
	req.Header.Set("User-Agent", "fear-and-greed/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, wrap.Errorf("binance klines request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, wrap.Errorf("read binance response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, wrap.Errorf("binance klines status %d: %s", resp.StatusCode, string(body))
	}

	var raw [][]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, wrap.Errorf("decode binance klines: %w", err)
	}

	out := make([]Kline, 0, len(raw))
	for _, row := range raw {
		k, err := parseKlineRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, k)
	}

	return out, nil
}

func parseKlineRow(row []json.RawMessage) (Kline, error) {
	if len(row) < 11 {
		return Kline{}, wrap.Errorf("unexpected kline row length %d", len(row))
	}

	openMs, err := parseInt64Field(row[0])
	if err != nil {
		return Kline{}, wrap.Errorf("open time: %w", err)
	}
	closeMs, err := parseInt64Field(row[6])
	if err != nil {
		return Kline{}, wrap.Errorf("close time: %w", err)
	}

	open, err := parseFloatField(row[1])
	if err != nil {
		return Kline{}, wrap.Errorf("open: %w", err)
	}
	high, err := parseFloatField(row[2])
	if err != nil {
		return Kline{}, wrap.Errorf("high: %w", err)
	}
	low, err := parseFloatField(row[3])
	if err != nil {
		return Kline{}, wrap.Errorf("low: %w", err)
	}
	closePrice, err := parseFloatField(row[4])
	if err != nil {
		return Kline{}, wrap.Errorf("close: %w", err)
	}
	volume, err := parseFloatField(row[5])
	if err != nil {
		return Kline{}, wrap.Errorf("volume: %w", err)
	}
	quoteVolume, err := parseFloatField(row[7])
	if err != nil {
		return Kline{}, wrap.Errorf("quote volume: %w", err)
	}
	trades, err := parseInt64Field(row[8])
	if err != nil {
		return Kline{}, wrap.Errorf("trades: %w", err)
	}
	takerBuyBase, err := parseFloatField(row[9])
	if err != nil {
		return Kline{}, wrap.Errorf("taker buy base: %w", err)
	}
	takerBuyQuote, err := parseFloatField(row[10])
	if err != nil {
		return Kline{}, wrap.Errorf("taker buy quote: %w", err)
	}

	return Kline{
		OpenTime:            time.UnixMilli(openMs).UTC(),
		Open:                open,
		High:                high,
		Low:                 low,
		Close:               closePrice,
		Volume:              volume,
		CloseTime:           time.UnixMilli(closeMs).UTC(),
		QuoteVolume:         quoteVolume,
		Trades:              trades,
		TakerBuyBaseVolume:  takerBuyBase,
		TakerBuyQuoteVolume: takerBuyQuote,
	}, nil
}

func parseInt64Field(raw json.RawMessage) (int64, error) {
	var v int64
	if err := json.Unmarshal(raw, &v); err == nil {
		return v, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, err
	}
	return strconv.ParseInt(s, 10, 64)
}

func parseFloatField(raw json.RawMessage) (float64, error) {
	var v float64
	if err := json.Unmarshal(raw, &v); err == nil {
		return v, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return 0, err
	}
	return strconv.ParseFloat(s, 64)
}

func intervalDuration(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "3m":
		return 3 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "2h":
		return 2 * time.Hour
	case "4h":
		return 4 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "8h":
		return 8 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "1d":
		return 24 * time.Hour
	case "3d":
		return 3 * 24 * time.Hour
	case "1w":
		return 7 * 24 * time.Hour
	default:
		return time.Minute
	}
}

// NormalizeSymbol converts BTC/USDT, btcusdt -> BTCUSDT.
func NormalizeSymbol(symbol string) (string, error) {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")
	if s == "" {
		return "", fmt.Errorf("empty symbol")
	}
	return s, nil
}

// CSVFilename returns e.g. BTCUSDT.csv or BTCUSDT_futures.csv.
func CSVFilename(symbol string, market Market) string {
	if market == MarketFutures {
		return symbol + "_futures.csv"
	}
	return symbol + ".csv"
}

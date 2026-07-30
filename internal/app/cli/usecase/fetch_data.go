package usecase

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/drybin/fear-and-greed/internal/infrastructure/binance"
	"github.com/drybin/fear-and-greed/pkg/progress"
	"github.com/drybin/fear-and-greed/pkg/wrap"
)

type IFetchData interface {
	Process(ctx context.Context, opts FetchDataOptions) error
}

type FetchDataOptions struct {
	Symbol     string
	Interval   string
	Market     binance.Market
	Dir        string
	Since      time.Time
	Until      time.Time
	NoProgress bool
}

type FetchData struct {
	binance *binance.Client
}

func NewFetchDataUsecase(client *binance.Client) *FetchData {
	return &FetchData{binance: client}
}

func (u *FetchData) Process(ctx context.Context, opts FetchDataOptions) error {
	symbol, err := binance.NormalizeSymbol(opts.Symbol)
	if err != nil {
		return wrap.Errorf("symbol: %w", err)
	}

	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return wrap.Errorf("create data dir: %w", err)
	}

	log.Printf("fetching %s %s %s from %s to %s",
		opts.Market, symbol, opts.Interval,
		opts.Since.Format(time.RFC3339),
		opts.Until.Format(time.RFC3339),
	)

	filename := binance.CSVFilename(symbol, opts.Market)
	path := filepath.Join(opts.Dir, filename)
	tmpPath := path + ".tmp"

	writer, err := newKlineCSVWriter(tmpPath)
	if err != nil {
		return err
	}
	defer func() { _ = writer.Close() }()

	estimated := binance.EstimatedCandles(opts.Since, opts.Until, opts.Interval)
	var bar *progress.Bar
	if !opts.NoProgress {
		desc := fmt.Sprintf("%s %s %s", opts.Market, symbol, opts.Interval)
		bar = progress.New(desc, estimated)
	}

	var onProgress binance.FetchProgress
	if bar != nil {
		var lastFetched int64
		onProgress = func(fetched, _ int64) {
			delta := int(fetched - lastFetched)
			if delta > 0 {
				_ = bar.Add(delta)
				lastFetched = fetched
			}
		}
	}

	total, err := u.binance.StreamKlines(
		ctx,
		opts.Market,
		symbol,
		opts.Interval,
		opts.Since,
		opts.Until,
		writer.WriteBatch,
		onProgress,
	)
	if err != nil {
		_ = os.Remove(tmpPath)
		return wrap.Errorf("fetch klines: %w", err)
	}
	if total == 0 {
		_ = os.Remove(tmpPath)
		return wrap.Errorf("no klines returned for %s", symbol)
	}

	if err := writer.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	if bar != nil {
		_ = bar.Finish()
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return wrap.Errorf("rename csv: %w", err)
	}

	log.Printf("saved %d candles to %s", total, path)
	return nil
}

type klineCSVWriter struct {
	file   *os.File
	writer *csv.Writer
}

func newKlineCSVWriter(path string) (*klineCSVWriter, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, wrap.Errorf("create csv: %w", err)
	}

	w := csv.NewWriter(f)
	header := []string{
		"open_time",
		"open",
		"high",
		"low",
		"close",
		"volume",
		"quote_volume",
		"trades",
		"taker_buy_base_volume",
		"taker_buy_quote_volume",
	}
	if err := w.Write(header); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, wrap.Errorf("write csv header: %w", err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, wrap.Errorf("flush csv header: %w", err)
	}

	return &klineCSVWriter{file: f, writer: w}, nil
}

func (kw *klineCSVWriter) WriteBatch(batch []binance.Kline) error {
	const timeLayout = "2006-01-02 15:04:05"
	for _, k := range batch {
		row := []string{
			k.OpenTime.UTC().Format(timeLayout),
			formatFloat(k.Open),
			formatFloat(k.High),
			formatFloat(k.Low),
			formatFloat(k.Close),
			formatFloat(k.Volume),
			formatFloat(k.QuoteVolume),
			strconv.FormatInt(k.Trades, 10),
			formatFloat(k.TakerBuyBaseVolume),
			formatFloat(k.TakerBuyQuoteVolume),
		}
		if err := kw.writer.Write(row); err != nil {
			return wrap.Errorf("write csv row: %w", err)
		}
	}
	kw.writer.Flush()
	return kw.writer.Error()
}

func (kw *klineCSVWriter) Close() error {
	if kw.writer != nil {
		kw.writer.Flush()
		if err := kw.writer.Error(); err != nil {
			return wrap.Errorf("flush csv: %w", err)
		}
	}
	if kw.file != nil {
		return kw.file.Close()
	}
	return nil
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// DefaultSince is the earliest sensible Binance spot history start.
func DefaultSince() time.Time {
	return time.Date(2017, 8, 17, 0, 0, 0, 0, time.UTC)
}

// ParseDateFlag parses YYYY-MM-DD in UTC.
func ParseDateFlag(s string) (time.Time, error) {
	t, err := time.ParseInLocation("2006-01-02", s, time.UTC)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse date %q: %w", s, err)
	}
	return t, nil
}

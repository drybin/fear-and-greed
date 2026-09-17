package usecase

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/drybin/fear-and-greed/internal/infrastructure/binance"
	"github.com/drybin/fear-and-greed/pkg/wrap"
)

type FetchFunding struct{ binance *binance.Client }

func NewFetchFundingUsecase(client *binance.Client) *FetchFunding {
	return &FetchFunding{binance: client}
}

func (u *FetchFunding) Process(ctx context.Context, symbol, dir string, since, until time.Time) error {
	normalized, err := binance.NormalizeSymbol(symbol)
	if err != nil {
		return wrap.Errorf("symbol: %w", err)
	}
	rates, err := u.binance.FundingRates(ctx, normalized, since, until)
	if err != nil {
		return err
	}
	if len(rates) == 0 {
		return fmt.Errorf("no funding rates returned for %s", normalized)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, binance.FundingCSVFilename(normalized))
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.Write([]string{"funding_time", "funding_rate"}); err != nil {
		_ = f.Close()
		return err
	}
	for _, rate := range rates {
		if err := w.Write([]string{rate.Time.UTC().Format("2006-01-02 15:04:05"), strconv.FormatFloat(rate.Rate, 'f', -1, 64)}); err != nil {
			_ = f.Close()
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

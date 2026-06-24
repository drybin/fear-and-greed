package scanreport_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/drybin/fear-and-greed/internal/domain/model"
	"github.com/drybin/fear-and-greed/internal/infrastructure/scanreport"
)

func TestBuildOHLC_andSave(t *testing.T) {
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var candles []model.Candle
	for i := 0; i < 480; i++ {
		p := 100 + float64(i%10)
		candles = append(candles, model.Candle{
			OpenTime: base.Add(time.Duration(i) * time.Minute),
			Open:     p, High: p + 1, Low: p - 1, Close: p + 0.5,
		})
	}
	dir := t.TempDir()
	r := scanreport.Result{
		Algo: "rise", Symbol: "BTCUSDT", Period: "full",
		CandleFrom: base.Format(time.RFC3339),
		CandleTo:   base.Add(8 * time.Hour).Format(time.RFC3339),
	}
	if err := scanreport.SaveOHLC(dir, r, candles, 60); err != nil {
		t.Fatal(err)
	}
	path := scanreport.OHLCPath(dir, r, 60)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestIsOHLCFile(t *testing.T) {
	if !scanreport.IsOHLCFile("BTCUSDT__full__ohlc_240m.json") {
		t.Fatal("expected ohlc file")
	}
	if scanreport.IsOHLCFile("BTCUSDT__full.json") {
		t.Fatal("expected result file")
	}
}

func TestLoadAllResults_skipsOHLC(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, scanreport.DataSubdir, "rise")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	result := `{"algo":"rise","symbol":"BTCUSDT","period":"full","best":{"profit_pct":1}}`
	if err := os.WriteFile(filepath.Join(dataDir, "BTCUSDT__full.json"), []byte(result), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "BTCUSDT__full__ohlc_240m.json"), []byte(`{"candles":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := scanreport.LoadAllResults(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
}

package scanreport_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drybin/fear-and-greed/internal/infrastructure/scanreport"
)

func TestClearAlgoSymbol_preservesOtherSymbols(t *testing.T) {
	dir := t.TempDir()
	w, err := scanreport.NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	algoDir := filepath.Join(dir, scanreport.DataSubdir, "rise")
	if err := os.MkdirAll(algoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite := func(name string) {
		if err := os.WriteFile(filepath.Join(algoDir, name), []byte(`{}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("BTCUSDT__full.json")
	mustWrite("BTCUSDT__full__ohlc_240m.json")
	mustWrite("ETHUSDT__full.json")
	mustWrite("ETHUSDT__full__ohlc_240m.json")

	if err := w.ClearAlgoSymbol("rise", "BTCUSDT"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(algoDir, "BTCUSDT__full.json")); !os.IsNotExist(err) {
		t.Fatalf("expected BTC result removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(algoDir, "BTCUSDT__full__ohlc_240m.json")); !os.IsNotExist(err) {
		t.Fatalf("expected BTC OHLC removed, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(algoDir, "ETHUSDT__full.json")); err != nil {
		t.Fatalf("ETH result should remain: %v", err)
	}
	if _, err := os.Stat(filepath.Join(algoDir, "ETHUSDT__full__ohlc_240m.json")); err != nil {
		t.Fatalf("ETH OHLC should remain: %v", err)
	}
}

func TestWriterSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	w, err := scanreport.NewWriter(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ClearAlgo("rise"); err != nil {
		t.Fatal(err)
	}
	r := scanreport.Result{
		Algo:        "rise",
		Symbol:      "BTCUSDT",
		Period:      "full",
		PeriodLabel: "весь период",
		CandleFrom:  "2024-01-01T00:00:00Z",
		CandleTo:    "2025-01-01T00:00:00Z",
		CandleCount: 100,
		Best: scanreport.Best{
			ParamLabel: "target 5%",
			ParamValue: 5,
			ProfitPct:  10.5,
			ProfitUSD:  10.5,
			TradeCount: 3,
		},
	}
	if err := w.Save(r); err != nil {
		t.Fatal(err)
	}
	if err := w.FinishManifest(map[string]interface{}{"test": true}); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, scanreport.DataSubdir, "rise", "BTCUSDT__full.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected result file: %v", err)
	}

	loaded, err := scanreport.LoadAllResults(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].Best.TradeCount != 3 {
		t.Fatalf("unexpected loaded: %+v", loaded)
	}

	htmlPath := filepath.Join(dir, "out.html")
	if err := scanreport.GenerateHTML(dir, htmlPath); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(htmlPath); err != nil || st.Size() < 100 {
		t.Fatalf("html not generated: %v", err)
	}
}

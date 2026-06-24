package scanreport_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drybin/fear-and-greed/internal/infrastructure/scanreport"
)

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

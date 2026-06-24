package binance

import (
	"testing"
	"time"
)

func TestNormalizeSymbol(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"btcusdt", "BTCUSDT"},
		{"BTC/USDT", "BTCUSDT"},
		{" eth-usdt ", "ETHUSDT"},
	}
	for _, tt := range tests {
		got, err := NormalizeSymbol(tt.in)
		if err != nil {
			t.Fatalf("%q: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("%q: got %q want %q", tt.in, got, tt.want)
		}
	}
}

func TestCSVFilename(t *testing.T) {
	if got := CSVFilename("BTCUSDT", MarketSpot); got != "BTCUSDT.csv" {
		t.Fatalf("spot: %s", got)
	}
	if got := CSVFilename("BTCUSDT", MarketFutures); got != "BTCUSDT_futures.csv" {
		t.Fatalf("futures: %s", got)
	}
}

func TestEstimatedCandles(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	if got := EstimatedCandles(start, end, "1h"); got != 24 {
		t.Fatalf("1h/day: got %d want 24", got)
	}
	if got := EstimatedCandles(start, end, "1m"); got != 1440 {
		t.Fatalf("1m/day: got %d want 1440", got)
	}
}

func TestParseMarket(t *testing.T) {
	m, err := ParseMarket("futures")
	if err != nil || m != MarketFutures {
		t.Fatalf("futures: %v %v", m, err)
	}
	m, err = ParseMarket("spot")
	if err != nil || m != MarketSpot {
		t.Fatalf("spot: %v %v", m, err)
	}
}

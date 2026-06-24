package usecase

import "testing"

func TestSymbolMatches(t *testing.T) {
	if !symbolMatches("BTCUSDT", "btcusdt") {
		t.Fatal("expected match")
	}
	if !symbolMatches("BTCUSDT", "BTCUSDT.csv") {
		t.Fatal("expected match with .csv suffix")
	}
	if symbolMatches("ETHUSDT", "BTCUSDT") {
		t.Fatal("expected no match")
	}
	if !symbolMatches("ETHUSDT", "") {
		t.Fatal("empty filter should match all")
	}
}

func TestNormalizeSMAReport(t *testing.T) {
	if normalizeSMAReport("all") != smaReportAll {
		t.Fatal("expected all")
	}
	if normalizeSMAReport("") != smaReportBest {
		t.Fatal("expected best default")
	}
}

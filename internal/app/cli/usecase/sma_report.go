package usecase

import (
	"fmt"
	"strings"

	"github.com/drybin/fear-and-greed/internal/strategy"
)

const (
	smaReportBest = "best"
	smaReportAll  = "all"
)

func normalizeSMAReport(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "all", "table", "full":
		return smaReportAll
	default:
		return smaReportBest
	}
}

func normalizeSymbolFilter(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return strings.ToUpper(strings.TrimSuffix(s, ".csv"))
}

func symbolMatches(symbol, filter string) bool {
	filter = normalizeSymbolFilter(filter)
	if filter == "" {
		return true
	}
	return normalizeSymbolFilter(symbol) == filter
}

type smaPerRow struct {
	smaPeriod  int
	longTarget int
	report     strategy.SimulationReport
}

func printSMAPerRowsTable(rows []smaPerRow) {
	fmt.Printf("    %6s  %7s  %10s  %10s  %8s\n", "SMA", "target", "profit %", "profit $", "trades")
	for _, row := range rows {
		fmt.Printf("    %6d  %6d%%  %10s  %10.2f  %8d\n",
			row.smaPeriod,
			row.longTarget,
			formatProfitPct(row.report),
			formatProfitUSD(row.report),
			row.report.CompletedCount,
		)
	}
}

package usecase

import (
	"fmt"

	"github.com/drybin/fear-and-greed/internal/infrastructure/scanreport"
)

// ResolveReportFlags normalizes --report-dir and --html CLI values.
func ResolveReportFlags(reportDir, htmlFlag string) (string, string, error) {
	dir, html, err := scanreport.ValidateReportSetup(reportDir, htmlFlag)
	if err != nil {
		return "", "", err
	}
	if html != "" && dir == "" {
		return "", "", fmt.Errorf("--html requires --report-dir (or use --html alone with default reports/)")
	}
	return dir, html, nil
}

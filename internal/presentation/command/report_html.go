package command

import (
	"fmt"
	"path/filepath"

	"github.com/drybin/fear-and-greed/internal/infrastructure/scanreport"
	"github.com/urfave/cli/v2"
)

// NewReportHTMLCommand regenerates HTML and backfills OHLC sidecars without re-running algos.
func NewReportHTMLCommand() *cli.Command {
	return &cli.Command{
		Name:  "report-html",
		Usage: "regenerate reports/report.html and OHLC chart data from existing JSON + data/*.csv",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "report-dir",
				Value: "reports",
				Usage: "report root (manifest + data/)",
			},
			&cli.StringFlag{
				Name:  "html",
				Value: "true",
				Usage: "output HTML path, or 'true' for <report-dir>/report.html",
			},
		},
		Action: func(c *cli.Context) error {
			reportDir, htmlPath, err := scanreport.ValidateReportSetup(c.String("report-dir"), c.String("html"))
			if err != nil {
				return err
			}
			if reportDir == "" {
				return fmt.Errorf("--report-dir is required")
			}
			if err := scanreport.GenerateHTML(reportDir, htmlPath); err != nil {
				return err
			}
			out := htmlPath
			if out == "" {
				out = scanreport.DefaultHTMLPath(reportDir)
			}
			fmt.Printf("HTML report: %s\n", out)
			fmt.Printf("Chart page:  %s\n", filepath.Join(reportDir, scanreport.DefaultChartHTML))
			fmt.Println("Open via HTTP server from project root, e.g.: python3 -m http.server 8000 → http://localhost:8000/reports/report.html")
			return nil
		},
	}
}

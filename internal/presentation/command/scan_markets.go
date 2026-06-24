package command

import (
	"context"

	"github.com/drybin/fear-and-greed/internal/app/cli/usecase"
	"github.com/drybin/fear-and-greed/internal/strategy"
	"github.com/urfave/cli/v2"
)

func NewScanMarketsCommand(service usecase.IScanMarkets) *cli.Command {
	return &cli.Command{
		Name:  "scan-markets",
		Usage: "random buy strategies on all CSV in data/; sweep sell target 1-100%",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "algo",
				Usage: "rise | rise-2d-profit | drop | drop-margin | trend | trend-long | trend-long-sma | trend-long-sma-retest | crt-long | breakout-retest-long | breakout-retest-long-v2 | fib-pullback-long | fib-pullback-long-v2 | fib-pullback-trend-v1 | nr7-trend-breakout-v1 | volatility-compression-breakout-v1 | liquidity-sweep-long | liquidity-sweep-long-v2 | liquidity-sweep-long-v3 | liquidity-sweep-long-v4 | liquidity-sweep-long-v5 | all; default all = rise+drop",
			},
			&cli.StringFlag{
				Name:  "dir",
				Value: "data",
				Usage: "directory with fetch-data CSV files",
			},
			&cli.Int64Flag{
				Name:  "seed",
				Value: strategy.DefaultSeed,
				Usage: "base RNG seed (each symbol/period gets a derived seed)",
			},
			&cli.Float64Flag{
				Name:  "last-years",
				Value: 2,
				Usage: "length of the short period in years",
			},
			&cli.IntFlag{
				Name:  "target-min",
				Value: 1,
				Usage: "min sell profit target percent",
			},
			&cli.IntFlag{
				Name:  "target-max",
				Value: 100,
				Usage: "max sell profit target percent",
			},
			&cli.IntFlag{
				Name:  "target-step",
				Value: 1,
				Usage: "sell target sweep step",
			},
			&cli.IntFlag{
				Name:  "min-trades",
				Value: 1,
				Usage: "min completed trades to count a run as best (excludes open-only positions)",
			},
			&cli.Float64Flag{
				Name:  "retest-epsilon",
				Value: strategy.TrendRetestEpsilonPct,
				Usage: "touch tolerance around SMA in percent for trend-long-sma-retest",
			},
			&cli.IntFlag{
				Name:  "retest-lookahead",
				Value: strategy.TrendRetestLookaheadCandles,
				Usage: "max candles after breakout to find retest (trend-long-sma-retest)",
			},
			&cli.StringFlag{
				Name:  "symbol",
				Usage: "run only this market (e.g. BTCUSDT); matches CSV filename",
			},
			&cli.StringFlag{
				Name:  "sma-report",
				Value: "best",
				Usage: "SMA sweep output: best (overall only) or all (table per SMA)",
			},
			&cli.StringFlag{
				Name:  "report-dir",
				Usage: "write JSON results to <dir>/data/<algo>/ (clears algo subdir each run)",
			},
			&cli.StringFlag{
				Name:  "html",
				Usage: "generate HTML comparison report (path, or 'true' for <report-dir>/report.html)",
			},
		},
		Action: func(c *cli.Context) error {
			algos, err := usecase.ParseAlgos(c.StringSlice("algo"))
			if err != nil {
				return err
			}
			reportDir, htmlPath, err := usecase.ResolveReportFlags(c.String("report-dir"), c.String("html"))
			if err != nil {
				return err
			}
			return service.Process(context.Background(), usecase.ScanMarketsOptions{
				Dir:                    c.String("dir"),
				Seed:                   c.Int64("seed"),
				LastYears:              c.Float64("last-years"),
				TargetMin:              c.Int("target-min"),
				TargetMax:              c.Int("target-max"),
				TargetStep:             c.Int("target-step"),
				MinTrades:              c.Int("min-trades"),
				RetestEpsilonPct:       c.Float64("retest-epsilon"),
				RetestLookaheadCandles: c.Int("retest-lookahead"),
				Symbol:                 c.String("symbol"),
				SMAReport:              c.String("sma-report"),
				ReportDir:              reportDir,
				HTMLPath:               htmlPath,
				Algos:                  algos,
			})
		},
	}
}

package command

import (
	"context"
	"time"

	"github.com/drybin/fear-and-greed/internal/app/cli/usecase"
	"github.com/drybin/fear-and-greed/internal/infrastructure/binance"
	"github.com/urfave/cli/v2"
)

func NewFetchDataCommand(service usecase.IFetchData) *cli.Command {
	return &cli.Command{
		Name:  "fetch-data",
		Usage: "download Binance OHLCV history to data/<symbol>.csv",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "symbol",
				Value: "BTCUSDT",
				Usage: "trading pair (BTCUSDT or BTC/USDT)",
			},
			&cli.StringFlag{
				Name:  "interval",
				Value: "1m",
				Usage: "candle interval (1m, 5m, 1h, 1d, ...)",
			},
			&cli.StringFlag{
				Name:  "market",
				Value: "spot",
				Usage: "spot or futures (USD-M perpetual)",
			},
			&cli.StringFlag{
				Name:  "dir",
				Value: "data",
				Usage: "output directory for CSV files",
			},
			&cli.StringFlag{
				Name:  "since",
				Value: "2017-08-17",
				Usage: "start date UTC (YYYY-MM-DD)",
			},
			&cli.StringFlag{
				Name:  "until",
				Usage: "end date UTC (YYYY-MM-DD), default now",
			},
			&cli.BoolFlag{
				Name:  "no-progress",
				Usage: "disable progress bar (for logs/CI)",
			},
		},
		Action: func(c *cli.Context) error {
			market, err := binance.ParseMarket(c.String("market"))
			if err != nil {
				return err
			}

			since, err := usecase.ParseDateFlag(c.String("since"))
			if err != nil {
				return err
			}

			until := time.Now().UTC()
			if c.IsSet("until") {
				until, err = usecase.ParseDateFlag(c.String("until"))
				if err != nil {
					return err
				}
				// include the full end day
				until = until.Add(24 * time.Hour)
			}

			return service.Process(context.Background(), usecase.FetchDataOptions{
				Symbol:     c.String("symbol"),
				Interval:   c.String("interval"),
				Market:     market,
				Dir:        c.String("dir"),
				Since:      since,
				Until:      until,
				NoProgress: c.Bool("no-progress"),
			})
		},
	}
}

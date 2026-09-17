package command

import (
	"context"
	"time"

	"github.com/drybin/fear-and-greed/internal/app/cli/usecase"
	"github.com/urfave/cli/v2"
)

func NewFetchFundingCommand(service *usecase.FetchFunding) *cli.Command {
	return &cli.Command{
		Name: "fetch-funding", Usage: "download Binance USD-M perpetual funding history",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "symbol", Required: true, Usage: "USD-M perpetual symbol, e.g. BTCUSDT"},
			&cli.StringFlag{Name: "dir", Value: "data", Usage: "output directory"},
			&cli.StringFlag{Name: "since", Required: true, Usage: "start date UTC (YYYY-MM-DD)"},
			&cli.StringFlag{Name: "until", Required: true, Usage: "exclusive end date UTC (YYYY-MM-DD)"},
		},
		Action: func(c *cli.Context) error {
			since, err := time.ParseInLocation("2006-01-02", c.String("since"), time.UTC)
			if err != nil {
				return err
			}
			until, err := time.ParseInLocation("2006-01-02", c.String("until"), time.UTC)
			if err != nil {
				return err
			}
			return service.Process(context.Background(), c.String("symbol"), c.String("dir"), since, until)
		},
	}
}

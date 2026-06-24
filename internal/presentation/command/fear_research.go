package command

import (
    "context"
    
    "github.com/drybin/fear-and-greed/internal/app/cli/usecase"
    "github.com/urfave/cli/v2"
)

func NewFearResearchCommand(service usecase.IFearResearch) *cli.Command {
    return &cli.Command{
        Name:  "fear-research",
        Usage: "fear-research command",
        Flags: []cli.Flag{},
        Action: func(c *cli.Context) error {
            return service.Process(context.Background())
        },
    }
}

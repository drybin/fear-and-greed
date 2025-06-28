package command

import (
    "context"
    
    "github.com/drybin/fead-and-greed/internal/app/cli/usecase"
    "github.com/urfave/cli/v2"
)

func NewHelloWorldCommand(service usecase.IHelloWorld) *cli.Command {
    return &cli.Command{
        Name:  "hello-world",
        Usage: "hello-world command",
        Flags: []cli.Flag{},
        Action: func(c *cli.Context) error {
            return service.Process(context.Background())
        },
    }
}

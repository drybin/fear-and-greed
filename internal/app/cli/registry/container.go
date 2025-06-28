package registry

import (
    "github.com/MinterTeam/minter-go-sdk/v2/api/http_client"
    "github.com/drybin/fead-and-greed/internal/adapter/webapi"
    "github.com/drybin/fead-and-greed/internal/app/cli/config"
    "github.com/drybin/fead-and-greed/internal/app/cli/usecase"
    "github.com/drybin/fead-and-greed/pkg/logger"
    "github.com/drybin/fead-and-greed/pkg/telegram"
    "github.com/drybin/fead-and-greed/pkg/wrap"
    "github.com/go-resty/resty/v2"
)

type Container struct {
    Logger   logger.ILogger
    Usecases *Usecases
    Clean    func()
}

type Usecases struct {
    HelloWorld *usecase.HelloWorld
}

func NewContainer(
    config *config.Config,
) (*Container, error) {
    
    container := Container{
        Logger: log,
        Usecases: &Usecases{
            HelloWorld: usecase.NewHelloWorldUsecase(),
        },
        Clean: func() {
        },
    }
    
    return &container, nil
}

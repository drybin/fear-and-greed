package registry

import (
	"github.com/drybin/fear-and-greed/internal/app/cli/config"
	"github.com/drybin/fear-and-greed/internal/app/cli/usecase"
	"github.com/drybin/fear-and-greed/internal/infrastructure/binance"
	"github.com/drybin/fear-and-greed/pkg/logger"
)

type Container struct {
    Logger   logger.ILogger
    Usecases *Usecases
    Clean    func()
}

type Usecases struct {
	HelloWorld   *usecase.HelloWorld
	FearResearch *usecase.FearResearch
	FetchData    *usecase.FetchData
	ScanMarkets  *usecase.ScanMarkets
}

func NewContainer(
    config *config.Config,
) (*Container, error) {
    
	binanceClient := binance.NewClient()
	container := Container{
		Usecases: &Usecases{
			HelloWorld:   usecase.NewHelloWorldUsecase(),
			FearResearch: usecase.NewFearResearchUsecase(),
			FetchData:    usecase.NewFetchDataUsecase(binanceClient),
			ScanMarkets:  usecase.NewScanMarketsUsecase(),
		},
		Clean: func() {},
	}
    
    return &container, nil
}

package main

import (
	"errors"
	"log"
	"os"

	"github.com/drybin/fear-and-greed/internal/app/cli"
	"github.com/drybin/fear-and-greed/internal/app/cli/config"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Println("failed to load env", err)
	}
    
    configObj, err := config.InitConfig()
    if err != nil {
        log.Fatal("failed to init cli config", err)
    }
    
    if err := cli.Run(configObj); err != nil {
        log.Fatal("failed to run cli app: ", err)
    }
}

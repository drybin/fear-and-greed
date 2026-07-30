package config

import (
    "errors"
    "time"
    
    "github.com/drybin/fear-and-greed/pkg/env"
)

type Config struct {
    ServiceName string
    PassPhrase  string
    TgConfig    TgConfig
}

type TgConfig struct {
    BotToken string
    ChatId   string
    Timeout  time.Duration
}

func (c Config) Validate() error {
	var errs []error
	if c.ServiceName == "" {
		errs = append(errs, errors.New("ServiceName is required"))
	}
	if c.PassPhrase == "" {
		errs = append(errs, errors.New("PassPhrase is required"))
	}
	return errors.Join(errs...)
}

func InitConfig() (*Config, error) {
    config := Config{
        ServiceName: env.GetString("APP_NAME", "fead-and-greed"),
        TgConfig:    initTgConfig(),
    }
    
    if err := config.Validate(); err != nil {
        return nil, err
    }
    
    return &config, nil
}

func initTgConfig() TgConfig {
    return TgConfig{
        BotToken: env.GetString("TG_BOT_TOKEN", ""),
        ChatId:   env.GetString("TG_CHAT_ID", ""),
        Timeout:  10 * time.Second,
    }
}

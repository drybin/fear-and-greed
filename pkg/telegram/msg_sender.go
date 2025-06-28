package telegram

import (
    "fmt"
    
    "github.com/drybin/fead-and-greed/pkg/wrap"
    "github.com/go-resty/resty/v2"
)

type TelegramWebapi struct {
    client   *resty.Client
    botToken string
    chatId   string
}

type TelegramWebapiMessage struct {
    Text   string `json:"text"`
    ChatId string `json:"chat_id"`
}

func NewTelegramWebapi(
    client *resty.Client,
    botToken string,
    chatId string,
) *TelegramWebapi {
    return &TelegramWebapi{
        client:   client,
        botToken: botToken,
        chatId:   chatId,
    }
}

func (c *TelegramWebapi) Send(msg string) (bool, error) {
    
    resp, err := c.client.R().
        SetHeader("Content-Type", "application/json").
        SetBody(TelegramWebapiMessage{
            ChatId: c.chatId,
            Text:   msg,
        }).
        Post(fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?parse_mode=HTML", c.botToken))
    
    if err != nil {
        return false, wrap.Errorf("failed to send tg message: %w", err)
    }
    
    if resp.StatusCode() != 200 {
        return false, wrap.Errorf("failed to send tg message: %s", resp.Body())
    }
    
    return true, nil
}

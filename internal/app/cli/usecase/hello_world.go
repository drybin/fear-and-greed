package usecase

import (
	"context"
	"log"
)

type IHelloWorld interface {
	Process(ctx context.Context) error
}

type HelloWorld struct {
}

func NewHelloWorldUsecase() *HelloWorld {
	return &HelloWorld{}
}

func (u *HelloWorld) Process(_ context.Context) error {
	log.Println("Hello World!!!")

	return nil
}

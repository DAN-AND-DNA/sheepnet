package main

import (
	"bytes"
	"github.com/dan-and-dna/sheepnet"
	"log"
	"sync"
	"time"
)

func main() {
	pool := &sync.Pool{
		New: func() any {
			return new(bytes.Buffer)
		},
	}
	logger := &DemoLogger{}

	client := sheepnet.NewClient(sheepnet.Config{}, sheepnet.WithLogger(logger))
	defer client.Stop()

	client.HookOnConnected(func(conn sheepnet.ConnWrapper) error {
		log.Println("connected")
		return nil
	})

	err := client.Dial("127.0.0.1:3737")
	if err != nil {
		panic(err)
	}

	message := pool.Get().(*bytes.Buffer)
	message.WriteString("DDD")
	err = client.SendAndReuse(message)
	if err != nil {
		panic(err)
	}

	time.Sleep(10 * time.Second)
}

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

	client := sheepnet.NewClient(1024, sheepnet.WithLogger(logger), sheepnet.WithBytesBufferPool(pool))

	client.HookOnConnected(func(conn sheepnet.ConnWrapper) error {
		log.Println("connected")
		return nil
	})

	err := client.Dial("127.0.0.1:3737")
	if err != nil {
		panic(err)
	}

	message := pool.Get().(*bytes.Buffer)
	message.Reset()
	message.WriteString("DDD")
	err = client.Send2(message, true)
	if err != nil {
		panic(err)
	}

	time.Sleep(3 * time.Second)
	client.Stop()
	time.Sleep(1 * time.Second)
}

package main

import (
	"bytes"
	"context"
	"github.com/dan-and-dna/sheepnet"
	"log"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	config := sheepnet.Config{
		Tcp4: []sheepnet.TCP4Config{
			{
				Address: "0.0.0.0:3737",
			},
		},
	}

	logger := &DemoLogger{}

	pool := &sync.Pool{
		New: func() any {
			return new(bytes.Buffer)
		},
	}

	server := sheepnet.NewServer(config, sheepnet.WithLogger(logger), sheepnet.WithBytesBufferPool(pool))
	server.HookOnConnected(func(conn sheepnet.ConnWrapper) error {
		log.Println("new conn")
		return nil
	})

	server.HookOnStop(func(conn sheepnet.ConnWrapper) {
		log.Println("conn stop")
	})

	onceByte := make([]byte, 1024)
	server.HookOnMessage(func(conn sheepnet.ConnWrapper) error {
		log.Println("get message")
		netConn := conn.GetNetConn()

		n, err := netConn.Read(onceByte)
		if err != nil {
			return err
		}

		log.Println(string(onceByte[:n]))

		return nil
	})

	err := server.Run()
	if err != nil {
		logger.ERR(err.Error())
	}
	defer server.Stop()

	select {
	case <-ctx.Done():
		logger.INFO("receive CTRL + C")
		logger.INFO("exiting...")
	}
}

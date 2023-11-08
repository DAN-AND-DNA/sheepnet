package main

import (
	"context"
	"github.com/dan-and-dna/sheepnet"
	"os/signal"
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

	r := &Router{}
	logger := &r.Logger

	server := sheepnet.NewServer(config, sheepnet.WithLogger(logger))
	server.SetRouter(r)
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

package internal

import (
	"bytes"
	"context"
	"net"
	"sync"
)

type Client struct {
	// 配置
	config Config

	address string

	wg         *sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	logger     Logger
	connection *Connection
	bytesPool  *sync.Pool

	// 钩子
	onConnected func(ConnWrapper) error // 刚连接上钩子
	onMessage   func(ConnWrapper) error // 刚消息钩子
	onStop      func(ConnWrapper)       // 刚关闭钩子

	sync.RWMutex
}

func NewClient(maxPendingMessages uint32, opts ...Option) ClientWrapper {
	client := &Client{}

	config := Config{
		MaxPendingMessages: maxPendingMessages,
	}

	client.config = config
	client.wg = &sync.WaitGroup{}
	client.ctx, client.cancel = context.WithCancel(context.Background())

	for _, opt := range opts {
		opt(client)
	}

	return client
}

func (client *Client) Dial(address string) error {
	if client == nil {
		return nil
	}

	client.address = address
	conn, err := net.Dial("tcp4", client.address)
	if err != nil {
		return err
	}

	client.connection = NewConnection(37, conn, client)

	if client.connection != nil {
		client.connection.Run()
	}

	go func() {
		select {
		case <-client.ctx.Done():
			if client.logger != nil {
				client.logger.INFO("start stop client conn")
			}

			client.finalizer()
		}
	}()

	return nil
}

func (client *Client) GetConn() ConnWrapper {
	return client.connection
}

func (client *Client) finalizer() {
	if client == nil {
		return
	}

	if client.connection != nil {
		client.connection.Stop()
	}
}

func (client *Client) Stop() {
	if client == nil {
		return
	}

	client.cancel()
}

func (client *Client) HookOnConnected(hooker func(conn ConnWrapper) error) {
	if client == nil {
		return
	}

	client.onConnected = hooker
}

func (client *Client) HookOnMessage(hooker func(conn ConnWrapper) error) {
	if client == nil {
		return
	}

	client.onMessage = hooker
}

func (client *Client) HookOnStop(hooker func(conn ConnWrapper)) {
	if client == nil {
		return
	}

	client.onStop = hooker
}

func (client *Client) Send1(message []byte) error {
	return client.connection.Send1(message)
}

func (client *Client) Send2(message *bytes.Buffer, reuse bool) error {
	return client.connection.Send2(message, reuse)
}

func (client *Client) GetLogger() Logger {
	return client.logger
}

func (client *Client) GetConfig() Config {
	if client == nil {
		return Config{}
	}

	return client.config
}

func (client *Client) GetOnMessage() func(ConnWrapper) error {
	if client == nil {
		return nil
	}

	return client.onMessage
}

func (client *Client) GetOnConnected() func(ConnWrapper) error {
	if client == nil {
		return nil
	}

	return client.onConnected
}

func (client *Client) GetOnStop() func(ConnWrapper) {
	if client == nil {
		return nil
	}

	return client.onStop
}

func (client *Client) GetBytesPool() *sync.Pool {
	if client == nil {
		return nil
	}

	return client.bytesPool
}

func (client *Client) GetWaitGroup() *sync.WaitGroup {
	if client == nil {
		return nil
	}

	return client.wg
}

func (client *Client) RemoveConnection(connId uint64) {
}

func (client *Client) SetLogger(logger Logger) {
	client.logger = logger
}

func (client *Client) SetBytesPool(pool *sync.Pool) {
	client.bytesPool = pool
}

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

func NewClient(config Config, opts ...Option) ClientWrapper {
	clientSide := &Client{}

	clientSide.config = config
	clientSide.wg = &sync.WaitGroup{}
	clientSide.ctx, clientSide.cancel = context.WithCancel(context.Background())

	return clientSide
}

func (client *Client) Dial(address string) error {
	conn, err := net.Dial("tcp4", address)
	if err != nil {
		return err
	}

	client.connection = NewConnection(37, conn, client)
	client.connection.Run()
	return nil
}

func (client *Client) Stop() {
	if client.connection != nil {
		client.connection.Stop()
	}
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

func (client *Client) SendAndReuse(message *bytes.Buffer) error {
	return client.connection.SendAndReuse(message)
}

func (client *Client) Send(message []byte) error {
	return client.connection.Send(message)
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

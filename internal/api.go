package internal

import (
	"bytes"
	"net"
	"sync"
)

// Logger 外部日志，需要外部保证并发安全
type Logger interface {
	ERR(string)
	INFO(string)
}

type Router interface {
	OnMessage(ConnWrapper) error
}

type ConnWrapper interface {
	GetNetConn() net.Conn
	SetError(error)
	GetError() error
	Send1(msg []byte) error
	Send2(msg *bytes.Buffer, reused bool) error
	Stop()
	InjectCtx(string, any)
	FetchCtx(string) any
}

type ServerWrapper interface {
	Run() error
	Stop()
	HookOnMessage(func(ConnWrapper) error)
	HookOnConnected(func(ConnWrapper) error)
	HookOnStop(func(ConnWrapper))
}

type ClientWrapper interface {
	Dial(address string) error
	Stop()
	HookOnMessage(func(ConnWrapper) error)
	HookOnConnected(func(ConnWrapper) error)
	HookOnStop(func(ConnWrapper))
	Send1(msg []byte) error
	Send2(msg *bytes.Buffer, reused bool) error
}

type Owner interface {
	SetLogger(Logger)
	GetLogger() Logger
	GetConfig() Config
	GetOnMessage() func(ConnWrapper) error
	GetOnConnected() func(ConnWrapper) error
	GetOnStop() func(ConnWrapper)
	GetBytesPool() *sync.Pool
	SetBytesPool(*sync.Pool)
	GetWaitGroup() *sync.WaitGroup
	RemoveConnection(uint64)
}

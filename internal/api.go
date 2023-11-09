package internal

import (
	"bytes"
	"net"
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
	Send(msg []byte) error
	Stop()
	AllocFastMsg() *bytes.Buffer
	SendFast(msg *bytes.Buffer) error
	InjectCtx(key string, value any)
	FetchCtx(key string) any
}

type ServerWrapper interface {
	Run() error
	Stop()
	HookOnMessage(func(ConnWrapper) error)
	HookOnConnected(func(ConnWrapper) error)
}

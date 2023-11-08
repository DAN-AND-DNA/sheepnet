package internal

import "net"

// Logger 外部日志，需要外部保证并发安全
type Logger interface {
	ERR(string)
	INFO(string)
}

type Router interface {
	OnNewConnection(ConnWrapper) error
}

type ConnWrapper interface {
	GetNetConn() net.Conn
	SetError(error)
	GetError() error
	Send(msg []byte) error
	Stop()
}

type ServerWrapper interface {
	SetRouter(router Router)
	Run() error
	Stop()
}

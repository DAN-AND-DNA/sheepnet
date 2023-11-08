package sheepnet

import (
	"github.com/dan-and-dna/sheepnet/internal"
)

// Logger 外部日志，需要外部保证并发安全
type Logger = internal.Logger

type Router = internal.Router

type ConnWrapper = internal.ConnWrapper

type Config = internal.Config
type TCP4Config = internal.TCP4Config

type Option = internal.Option

type ServerWrapper = internal.ServerWrapper

// WithLogger 添加日志
func WithLogger(logger Logger) Option {
	return internal.WithLogger(logger)
}

// NewServer 创建server
func NewServer(config Config, opts ...Option) ServerWrapper {
	return internal.NewServer(config, opts...)
}

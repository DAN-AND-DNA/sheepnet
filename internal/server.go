package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// 保证接口实现
	_ ServerWrapper = (*Server)(nil)
)

type Server struct {
	// 配置
	config Config

	ctx    context.Context
	cancel context.CancelFunc
	logger Logger // 外部logger的弱引用

	// 连接管理
	connId      atomic.Uint64
	connections sync.Map
	wg          *sync.WaitGroup

	// 路由
	router Router
}

func NewServer(config Config, opts ...Option) *Server {
	s := new(Server)
	s.config = config
	s.connId.Store(0)
	s.wg = &sync.WaitGroup{}

	for _, opt := range opts {
		opt(s)
	}

	s.ctx, s.cancel = context.WithCancel(context.Background())

	return s
}

func (s *Server) SetRouter(router Router) {
	s.router = router
}

func (s *Server) Run() error {
	if s == nil {
		return nil
	}
	// 先解析服务器配置
	var err error
	s.config, err = parseServerConfig(s.config)
	if err != nil {
		return err
	}

	if s.logger != nil {
		buf, _ := json.MarshalIndent(s.config, "", "	")
		s.logger.INFO("\n" + string(buf))
	}

	// 监听tcp4请求
	if len(s.config.Tcp4) > 0 {
		err = s.listenTcp4()
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) Stop() {
	if s == nil {
		return
	}

	// 通知全部子协程退出
	s.cancel()

	// 等全部资源释放
	s.wg.Wait()
}

func (s *Server) listenTcp4() error {
	if s == nil {
		return nil
	}

	if len(s.config.Tcp4) == 0 {
		return nil
	}

	// 全部 tcp listener
	var ls []*net.TCPListener
	for _, tcp4Config := range s.config.Tcp4 {
		l, err := net.ListenTCP("tcp4", tcp4Config.addressResolved)
		if err != nil {
			return err
		}
		ls = append(ls, l)
	}

	for _, l := range ls {
		listener := l
		go func(l *net.TCPListener) {
			for {
				conn, err := l.Accept()
				if err != nil {
					if errors.Is(err, net.ErrClosed) {
						// 正常关闭监听
						return
					}

					if s.logger != nil {
						s.logger.ERR(fmt.Sprintf("listener accept error: %v", err))
					}

					// 失败延迟
					time.Sleep(5 * time.Millisecond)
					continue
				}

				// 包装tcp连接
				connId := s.connId.Add(1)

				connWrapper := NewConnection(connId, conn, s)
				if connWrapper != nil {
					s.connections.Store(connId, connWrapper)
					// 启动连接逻辑
					connWrapper.Run()
				}
			}

		}(listener)
	}

	go func() {
		select {
		case <-s.ctx.Done():
			if s.logger != nil {
				s.logger.INFO("start stop all listeners")
			}

			// 关闭listener，accept的阻塞退出，保证没新的连接
			for _, l := range ls {
				err := l.Close()
				if err != nil && s.logger != nil {
					s.logger.ERR(fmt.Sprintf("close listener error: %v", err))
				}
			}

			// 清理资源
			s.finalizer()
		}
	}()

	return nil
}

func (s *Server) finalizer() {
	if s == nil {
		return
	}

	s.connections.Range(func(key, value any) bool {
		if _, ok := key.(uint64); ok {
			if connection, ok := value.(*Connection); ok {
				connection.Stop()
			}
		}

		return true
	})
}

type Option func(*Server)

func WithLogger(logger Logger) Option {
	return func(server *Server) {
		server.logger = logger
	}
}

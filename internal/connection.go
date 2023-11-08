package internal

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

var (
	// 保证接口实现
	_ ConnWrapper = (*Connection)(nil)
)

type Connection struct {
	isClosed           bool
	conn               net.Conn
	connId             uint64
	ctx                context.Context
	cancel             context.CancelFunc
	logger             Logger      // 外部logger
	server             *Server     // 从属的server
	sendChan           chan []byte // 发送消息队列
	maxPendingMessages uint32      // 最大阻塞未发消息
	router             Router
	err                error
	errMutex           sync.Mutex
	sync.RWMutex
}

func (c *Connection) GetNetConn() net.Conn {
	if c == nil {
		return nil
	}

	return c.conn
}

func (c *Connection) SetError(err error) {
	if c == nil {
		return
	}

	c.errMutex.Lock()
	defer c.errMutex.Unlock()

	c.err = err
}

func (c *Connection) GetError() error {
	if c == nil {
		return nil
	}

	return c.err
}

func NewConnection(connId uint64, conn net.Conn, s *Server) *Connection {
	if s == nil {
		return nil
	}

	c := new(Connection)
	c.conn = conn
	c.connId = connId
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.logger = s.logger
	c.server = s
	c.maxPendingMessages = s.config.MaxPendingMessages
	c.router = s.router
	c.sendChan = make(chan []byte, 1000)

	return c
}

func (c *Connection) Run() {
	if c == nil {
		return
	}

	// 捕捉异常
	defer func() {
		if err := recover(); err != nil {
			if c.logger != nil {
				c.logger.ERR(fmt.Sprintf("connection run error: %v", err))
			}
		}
	}()

	c.RLock()
	defer c.RUnlock()
	if c.isClosed {
		return
	}

	// 读协程
	go c.loopRead()
	// 写协程
	go c.loopWrite()

	// 清理资源
	go func() {
		c.server.wg.Add(1)
		defer c.server.wg.Done()

		select {
		case <-c.ctx.Done():
			c.finalizer()
		}
	}()
}

func (c *Connection) Stop() {
	if c == nil {
		return
	}

	// 已经关闭
	c.RLock()
	defer c.RUnlock()
	if c.isClosed {
		return
	}

	// 通知连接相关的全部协程
	c.cancel()
}

func (c *Connection) finalizer() {
	if c == nil {
		return
	}

	c.Lock()
	defer c.Unlock()
	if c.isClosed {
		return
	}

	c.isClosed = true

	// 取消注册conn到server
	c.server.connections.Delete(c.connId)

	// 关闭连接，使连接读写操作退出阻塞
	if c.conn != nil {
		_ = c.conn.Close()
	}

	// 关闭发送管道
	if c.sendChan != nil {
		close(c.sendChan)
	}
}

func (c *Connection) loopRead() {
	if c == nil {
		return
	}

	// 捕捉异常
	defer func() {
		if err := recover(); err != nil {
			if c.logger != nil {
				c.err = fmt.Errorf("connection loop read error: %v", err)
				c.logger.ERR(c.err.Error())
			}
		}
	}()

	c.server.wg.Add(1)
	defer c.server.wg.Done()
	defer c.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if c.router != nil {
				// 通知路由有新连接
				err := c.router.OnMessage(c)
				if err != nil {
					if c.logger != nil {
						c.logger.ERR(fmt.Sprintf("connection call router OnNewConnection error: %v", err))
					}

					// 保存错误方便回溯
					c.SetError(err)
					return
				}
			} else {
				// 默认是全读
				message, err := c.defaultOnMessage()
				if err != nil {
					if c.logger != nil {
						c.logger.ERR(fmt.Sprintf("connection call defaultOnNewConnection error: %v", err))
					}

					// 保存错误方便回溯
					c.SetError(err)
					return
				}

				// 默认是打印长度
				c.defaultDispatch(message)
			}
		}
	}
}

func (c *Connection) loopWrite() {
	if c == nil {
		return
	}

	// 捕捉异常
	defer func() {
		if err := recover(); err != nil {
			err := fmt.Errorf("connection loop write error: %v", err)
			c.SetError(err)
			if c.logger != nil {
				c.logger.ERR(c.err.Error())
			}
		}
	}()

	c.server.wg.Add(1)
	defer c.server.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case msg, ok := <-c.sendChan:
			if !ok {
				// chan 提前被关闭
				break
			}

			// 发送
			_, err := c.conn.Write(msg)
			if err != nil {
				c.SetError(err)
				if c.logger != nil {
					err = fmt.Errorf("connection loop write error: %v", err)
					c.logger.ERR(c.err.Error())
					return
				}
			}
		}
	}
}

func (c *Connection) Send(msg []byte) error {
	if c == nil {
		return nil
	}

	c.RLock()
	defer c.RUnlock()
	if c.isClosed {
		return nil
	}

	waitTimer := time.NewTimer(7 * time.Millisecond)
	defer waitTimer.Stop()

	select {
	case <-waitTimer.C:
		err := fmt.Errorf("connection send msg timeout")
		c.SetError(err)
		return err
	case c.sendChan <- msg:
		return nil
	}
}

func (c *Connection) defaultOnMessage() ([]byte, error) {
	message, err := io.ReadAll(c.conn)
	if err != nil {
		return nil, err
	}

	return message, nil
}

func (c *Connection) defaultDispatch(message []byte) {
	// 仅仅作调试
	if c.logger != nil {
		c.logger.INFO(fmt.Sprintf("connection defaultOnNewConnection read %d bytes from xxx", len(message)))
	}
}

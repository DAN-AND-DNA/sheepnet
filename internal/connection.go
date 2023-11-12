package internal

import (
	"bytes"
	"context"
	"errors"
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

func init() {
	go func() {
		for {
			time.Sleep(1 * time.Second)
		}
	}()
}

type Connection struct {
	isClosed           bool
	conn               net.Conn
	connId             uint64
	ctx                context.Context
	cancel             context.CancelFunc
	logger             Logger             // 外部logger
	sendChan           chan []byte        // 发送消息队列
	sendChanReused     chan *bytes.Buffer // 发送消息队列
	maxPendingMessages uint32             // 最大阻塞未发消息
	err                error
	errMutex           sync.Mutex
	putBackBytesPool   *sync.Pool              // 外部回收池子
	onConnected        func(ConnWrapper) error // 刚连接上钩子
	onMessage          func(ConnWrapper) error // 刚连接上钩子
	onStop             func(ConnWrapper)
	Ctx                sync.Map
	owner              Owner
	wg                 *sync.WaitGroup
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

func NewConnection(connId uint64, conn net.Conn, owner Owner) *Connection {
	if owner == nil {
		panic("new connection has no owner")
	}

	c := new(Connection)
	c.conn = conn
	c.connId = connId
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.logger = owner.GetLogger()
	c.maxPendingMessages = owner.GetConfig().MaxPendingMessages
	c.onMessage = owner.GetOnMessage()
	c.onConnected = owner.GetOnConnected()
	c.onStop = owner.GetOnStop()
	c.sendChan = make(chan []byte, c.maxPendingMessages)
	c.sendChanReused = make(chan *bytes.Buffer, c.maxPendingMessages)
	c.putBackBytesPool = owner.GetBytesPool()
	c.owner = owner
	c.wg = owner.GetWaitGroup()

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
		c.wg.Add(1)
		defer c.wg.Done()

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

	if c.onStop != nil {
		c.onStop(c)
	}

	c.Lock()
	defer c.Unlock()
	if c.isClosed {
		return
	}

	c.isClosed = true

	// 取消注册conn到server
	c.owner.RemoveConnection(c.connId)

	// 关闭连接，使连接读写操作退出阻塞
	if c.conn != nil {
		_ = c.conn.Close()
	}

	// 关闭发送管道
	if c.sendChan != nil {
		close(c.sendChan)
	}

	// 关闭发送管道
	if c.sendChanReused != nil {
		close(c.sendChanReused)
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
	
	c.wg.Add(1)
	defer c.wg.Done()
	defer c.Stop()

	if c.onConnected != nil {
		err := c.onConnected(c)
		if err != nil {
			if c.logger != nil {
				c.logger.ERR(fmt.Sprintf("connection onConnected error: %v", err))
			}

			// 保存错误方便回溯
			c.SetError(err)
			return
		}
	}

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			if c.onMessage != nil {
				// 通知路由有新连接
				err := c.onMessage(c)
				if err != nil {
					if c.logger != nil {
						if !errors.Is(err, io.EOF) {
							c.logger.ERR(fmt.Sprintf("connection onMessage error: %v", err))
						}
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
						if !errors.Is(err, net.ErrClosed) {
							c.logger.ERR(fmt.Sprintf("connection call defaultOnNewConnection error: %v", err))
						}
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

	c.wg.Add(1)
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case msg, ok := <-c.sendChan:
			if !ok {
				// chan 提前被关闭
				return
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
		case msg, ok := <-c.sendChanReused:
			if !ok {
				// chan 提前被关闭
				return
			}

			// 发送
			_, err := c.conn.Write(msg.Bytes())
			if c.putBackBytesPool != nil {
				msg.Reset()
				c.putBackBytesPool.Put(msg)
			}
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

func (c *Connection) Send1(msg []byte) error {
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

func (c *Connection) Send2(msg *bytes.Buffer, reused bool) error {
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
	case c.sendChanReused <- msg:
		return nil
	}
}

func (c *Connection) defaultOnMessage() ([]byte, error) {
	onceBuf := make([]byte, 30)
	_, err := c.conn.Read(onceBuf)
	if err != nil {
		return nil, err
	}

	return onceBuf, nil
}

func (c *Connection) defaultDispatch(message []byte) {
	// 仅仅作调试
	if c.logger != nil {
		c.logger.INFO(fmt.Sprintf("connection defaultOnNewConnection read %d bytes from xxx", len(message)))
	}
}

func (c *Connection) HookOnConnected(hooker func(conn ConnWrapper) error) {
	if c == nil {
		return
	}

	c.onConnected = hooker
}

func (c *Connection) HookOnMessage(hooker func(conn ConnWrapper) error) {
	if c == nil {
		return
	}

	c.onMessage = hooker
}

func (c *Connection) HookOnStop(hooker func(conn ConnWrapper)) {
	if c == nil {
		return
	}

	c.onStop = hooker
}

func (c *Connection) InjectCtx(key string, value any) {
	if c == nil {
		return
	}

	c.Ctx.Store(key, value)
}

func (c *Connection) FetchCtx(key string) any {
	if c == nil {
		return nil
	}

	value, ok := c.Ctx.Load(key)
	if !ok {
		return nil
	}

	return value
}

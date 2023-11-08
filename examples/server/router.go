package main

import (
	"bytes"
	"encoding/binary"
	"github.com/dan-and-dna/sheepnet"
	"io"
	"log"
	"reflect"
	"sync"
)

type Router struct {
	in       bytes.Buffer
	routes   map[uint16]Route
	messages map[uint16]*sync.Pool
	Logger   DemoLogger
}

type Route func(sheepnet.ConnWrapper, any, error)

func (r *Router) AddRoute(msgId uint16, route Route, message any) {
	// 检查回调的参数数量和返回数量
	messageType := reflect.TypeOf(message)
	if messageType.Kind() != reflect.Pointer || messageType.Elem().Kind() != reflect.Struct {
		panic("message should be pointer of a struct")
	}

	// 注册请求
	r.messages[msgId] = &sync.Pool{
		New: func() any {
			return reflect.New(messageType)
		},
	}

	// 注册回调
	r.routes[msgId] = route
}

type Message struct {
	Id   uint16
	Body []byte
}

func (r *Router) OnNewConnection(c sheepnet.ConnWrapper) error {
	if c == nil {
		return nil
	}

	// 直接发送
	err := c.Send([]byte("hello"))
	if err != nil {
		return err
	}

	conn := c.GetNetConn()

	var headLen int64 = 6

	n, err := r.in.ReadFrom(io.LimitReader(conn, headLen))
	if err != nil {
		return err
	}

	log.Println(n)

	// 解析包长度
	msgTotalSize := binary.LittleEndian.Uint32(r.in.Next(4))
	msgId := binary.LittleEndian.Uint16(r.in.Next(2))

	log.Println(msgId)
	log.Println(msgTotalSize)

	if int64(msgTotalSize) > headLen {
		_, err := r.in.ReadFrom(io.LimitReader(conn, int64(msgTotalSize)-headLen))
		if err != nil {
			return err
		}
	}

	// 二进制消息体
	msgBody := r.in.Next(int(msgTotalSize) - int(headLen))

	message := &Message{Id: msgId, Body: msgBody}
	r.Dispatch(c, message)

	return nil
}

func (r *Router) Dispatch(c sheepnet.ConnWrapper, input any) {
	if input == nil {
		return
	}

	newMessage := input.(*Message)
	msgId := newMessage.Id
	msgBody := newMessage.Body

	// 查找消息id对应的路由
	route, ok := r.routes[msgId]
	if !ok {
		return
	}

	log.Println(len(msgBody))

	// 从池里拿消息id对应的结构体
	messageValue := r.messages[msgId].Get().(reflect.Value)
	//  反序列化
	message := messageValue.Interface()
	_ = message
	var err error

	// 消息处理
	route(c, message, err)

	// 回收消息
	messageValue.Elem().SetZero()
	r.messages[msgId].Put(messageValue)
}

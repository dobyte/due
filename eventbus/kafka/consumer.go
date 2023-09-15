package kafka

import (
	"context"
	"github.com/symsimmy/due/eventbus"
	"github.com/symsimmy/due/log"
	"github.com/symsimmy/due/task"
	"reflect"
	"sync"
)

type consumer struct {
	ctx      context.Context
	cancel   context.CancelFunc
	rw       sync.RWMutex
	handlers map[uintptr]eventbus.EventHandler
}

// 添加处理器
func (c *consumer) addHandler(handler eventbus.EventHandler) int {
	pointer := reflect.ValueOf(handler).Pointer()

	c.rw.Lock()
	defer c.rw.Unlock()

	c.handlers[pointer] = handler

	return len(c.handlers)
}

// 移除处理器
func (c *consumer) remHandler(handler eventbus.EventHandler) int {
	pointer := reflect.ValueOf(handler).Pointer()

	c.rw.Lock()
	defer c.rw.Unlock()

	delete(c.handlers, pointer)

	return len(c.handlers)
}

// 分发数据
func (c *consumer) dispatch(data []byte) {
	event, err := eventbus.UnpackData(data)
	if err != nil {
		log.Error("invalid event data")
		return
	}

	c.rw.RLock()
	defer c.rw.RUnlock()

	for _, handler := range c.handlers {
		fn := handler
		task.AddTask(func() { fn(event) })
	}
}

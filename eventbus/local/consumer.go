package local

import (
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/task"
	"github.com/nats-io/nats.go"
	"reflect"
	"sync"
)

type consumer struct {
	sub      *nats.Subscription
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
func (c *consumer) dispatch(event *eventbus.Event) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	for _, handler := range c.handlers {
		fn := handler
		task.AddTask(func() { fn(event) })
	}
}

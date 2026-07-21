package redis

import (
	"sync"

	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
)

type consumer struct {
	eb   *Eventbus
	rw   sync.RWMutex
	subs []*subscription
}

func newConsumer(eb *Eventbus) *consumer {
	return &consumer{eb: eb, subs: make([]*subscription, 0, 1)}
}

// addSubscription 添加订阅
func (c *consumer) addSubscription(handler eventbus.EventHandler) *subscription {
	sub := &subscription{consumer: c, handler: handler}

	c.rw.Lock()
	c.subs = append(c.subs, sub)
	c.rw.Unlock()

	return sub
}

// delSubscription 移除订阅
func (c *consumer) delSubscription(sub *subscription) (found bool, empty bool) {
	c.rw.Lock()
	defer c.rw.Unlock()

	subs := make([]*subscription, 0, len(c.subs))
	for _, s := range c.subs {
		if s == sub {
			found = true
		} else {
			subs = append(subs, s)
		}
	}
	c.subs = subs

	return found, len(c.subs) == 0
}

// 分发数据
func (c *consumer) dispatch(data []byte) {
	event, err := c.eb.deserialize(data)
	if err != nil {
		log.Errorf("invalid event data: %v", err)
		return
	}

	c.rw.RLock()
	defer c.rw.RUnlock()

	for _, sub := range c.subs {
		handler := sub.handler
		if handler == nil {
			continue
		}
		task.AddTask(func() { handler(event) })
	}
}

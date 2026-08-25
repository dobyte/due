package process

import (
	"sync"

	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/utils/xrand"
)

type consumer struct {
	balance       bool
	rw            sync.RWMutex
	subscriptions []*subscription
}

// 添加订阅
func (c *consumer) addSubscription(topic string, handler eventbus.EventHandler) *subscription {
	sub := &subscription{topic: topic, handler: handler}

	c.rw.Lock()
	c.subscriptions = append(c.subscriptions, sub)
	c.rw.Unlock()

	return sub
}

// 移除订阅
func (c *consumer) delSubscription(sub *subscription) (found bool, empty bool) {
	c.rw.Lock()
	defer c.rw.Unlock()

	subscriptions := make([]*subscription, 0, len(c.subscriptions))
	for _, s := range c.subscriptions {
		if s == sub {
			found = true
		} else {
			subscriptions = append(subscriptions, s)
		}
	}
	c.subscriptions = subscriptions

	return found, len(c.subscriptions) == 0
}

// 分发数据
func (c *consumer) dispatch(event *eventbus.Event) {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if c.balance {
		if len(c.subscriptions) == 0 {
			return
		}

		sub := c.subscriptions[xrand.IntR(0, len(c.subscriptions)-1)]

		if sub.handler == nil {
			return
		}

		task.Add(func() { sub.handler(event) })
	} else {
		for _, sub := range c.subscriptions {
			handler := sub.handler

			if handler == nil {
				continue
			}

			task.Add(func() { handler(event) })
		}
	}
}

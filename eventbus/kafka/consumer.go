package kafka

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/IBM/sarama"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xtime"
)

type consumer struct {
	eb            *Eventbus
	ctx           context.Context
	cancel        context.CancelFunc
	rw            sync.RWMutex
	subscriptions []*subscription
	balance       bool
	idx           uint64
	consumer      sarama.Consumer
	group         sarama.ConsumerGroup
	groupID       string
	wg            sync.WaitGroup
}

func newConsumer(eb *Eventbus, balance bool) *consumer {
	c := &consumer{eb: eb, balance: balance, subscriptions: make([]*subscription, 0, 1)}
	c.ctx, c.cancel = context.WithCancel(eb.ctx)
	return c
}

// addSubscription 添加订阅
func (c *consumer) addSubscription(handler eventbus.EventHandler) *subscription {
	sub := &subscription{consumer: c, handler: handler}

	c.rw.Lock()
	c.subscriptions = append(c.subscriptions, sub)
	c.rw.Unlock()

	return sub
}

// delSubscription 移除订阅
func (c *consumer) delSubscription(sub *subscription) (found bool, empty bool) {
	c.rw.Lock()
	defer c.rw.Unlock()

	subs := make([]*subscription, 0, len(c.subscriptions))
	for _, s := range c.subscriptions {
		if s == sub {
			found = true
		} else {
			subs = append(subs, s)
		}
	}
	c.subscriptions = subs

	return found, len(c.subscriptions) == 0
}

// startBroadcastConsumer 启动广播模式消费
func (c *consumer) startBroadcastConsumer(consumer sarama.Consumer, topic string) error {
	partitions, err := consumer.Partitions(topic)
	if err != nil {
		return err
	}

	c.consumer = consumer

	for _, partition := range partitions {
		c.wg.Add(1)
		go c.runPartitionConsumer(topic, partition)
	}

	return nil
}

// runPartitionConsumer 运行单个 partition 的消费循环（带重试）
func (c *consumer) runPartitionConsumer(topic string, partition int32) {
	defer c.wg.Done()

	backoff := time.Millisecond * 100
	maxBackoff := time.Second * 10

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			pc, err := c.consumer.ConsumePartition(topic, partition, sarama.OffsetNewest)
			if err != nil {
				log.Errorf("kafka partition consumer create error, partition: %d, error: %v", partition, err)

				select {
				case <-c.ctx.Done():
					return
				case <-time.After(backoff):
				}

				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}

				continue
			}

			backoff = time.Millisecond * 100

		LOOP:
			for {
				select {
				case <-c.ctx.Done():
					pc.AsyncClose()
					return
				case msg := <-pc.Messages():
					c.dispatch(msg.Value)
				case err := <-pc.Errors():
					log.Errorf("kafka partition consumer error, partition: %d, error: %v", partition, err)
					pc.AsyncClose()
					break LOOP
				}
			}
		}
	}
}

// startGroupConsumer 启动消费组模式消费
func (c *consumer) startGroupConsumer(group sarama.ConsumerGroup, topic string) {
	c.group = group

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		backoff := time.Millisecond * 100
		maxBackoff := time.Second * 10

		for {
			select {
			case <-c.ctx.Done():
				return
			default:
				if err := group.Consume(c.ctx, []string{topic}, c); err != nil {
					if err == sarama.ErrClosedConsumerGroup {
						return
					}
					log.Errorf("kafka consumer group consume error: %v", err)

					select {
					case <-c.ctx.Done():
						return
					case <-time.After(backoff):
					}

					backoff *= 2
					if backoff > maxBackoff {
						backoff = maxBackoff
					}
				} else {
					backoff = time.Millisecond * 100
				}
			}
		}
	}()
}

// Setup 实现 sarama.ConsumerGroupHandler
func (c *consumer) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup 实现 sarama.ConsumerGroupHandler
func (c *consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim 实现 sarama.ConsumerGroupHandler
func (c *consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		c.dispatch(msg.Value)

		session.MarkMessage(msg, "")
	}
	return nil
}

// stop 停止消费
func (c *consumer) stop() {
	c.cancel()
	c.wg.Wait()
}

// 分发数据
func (c *consumer) dispatch(data []byte) {
	event, err := c.eb.deserialize(data)
	if err != nil {
		log.Errorf("invalid event data: %v", err)
		return
	}

	if c.eb.opts.staleDuration > 0 && xtime.Now().Sub(event.Timestamp) > c.eb.opts.staleDuration {
		log.Debugf("skip stale event, id: %s, topic: %s, timestamp: %v", event.ID, event.Topic, event.Timestamp)
		return
	}

	for _, handler := range c.loadHandlers() {
		xcall.Call(func() {
			handler(event)
		})
	}
}

// 加载订阅的事件处理函数
func (c *consumer) loadHandlers() []eventbus.EventHandler {
	c.rw.RLock()
	defer c.rw.RUnlock()

	if len(c.subscriptions) == 0 {
		return nil
	}

	if c.balance {
		idx := atomic.AddUint64(&c.idx, 1) % uint64(len(c.subscriptions))
		handler := c.subscriptions[idx].handler

		if handler == nil {
			return nil
		}

		return []eventbus.EventHandler{handler}
	} else {
		handlers := make([]eventbus.EventHandler, 0, len(c.subscriptions))

		for _, sub := range c.subscriptions {
			if sub.handler != nil {
				handlers = append(handlers, sub.handler)
			}
		}

		return handlers
	}
}

package kafka

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/IBM/sarama"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
)

type consumer struct {
	eb          *Eventbus
	ctx         context.Context
	cancel      context.CancelFunc
	rw          sync.RWMutex
	subs        []*subscription
	balance     bool
	idx         uint64
	consumer    sarama.Consumer
	group       sarama.ConsumerGroup
	groupID     string
	partitions  []int32
	partitionPC []sarama.PartitionConsumer
}

func newConsumer(eb *Eventbus, balance bool) *consumer {
	c := &consumer{eb: eb, balance: balance, subs: make([]*subscription, 0, 1)}
	c.ctx, c.cancel = context.WithCancel(eb.ctx)
	return c
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

// startBroadcastConsumer 启动广播模式消费
func (c *consumer) startBroadcastConsumer(consumer sarama.Consumer, topic string) error {
	partitions, err := consumer.Partitions(topic)
	if err != nil {
		return err
	}

	c.consumer = consumer
	c.partitions = partitions
	c.partitionPC = make([]sarama.PartitionConsumer, 0, len(partitions))

	for _, partition := range partitions {
		pc, err := consumer.ConsumePartition(topic, partition, sarama.OffsetNewest)
		if err != nil {
			return err
		}
		c.partitionPC = append(c.partitionPC, pc)

		go func(pc sarama.PartitionConsumer) {
			defer pc.AsyncClose()

			for {
				select {
				case <-c.ctx.Done():
					return
				case msg := <-pc.Messages():
					c.dispatch(msg.Value)
				case <-pc.Errors():
					return
				}
			}
		}(pc)
	}

	return nil
}

// startGroupConsumer 启动消费组模式消费
func (c *consumer) startGroupConsumer(group sarama.ConsumerGroup, topic string) {
	c.group = group

	go func() {
		for {
			select {
			case <-c.ctx.Done():
				return
			default:
				if err := group.Consume(c.ctx, []string{topic}, c); err != nil {
					log.Errorf("kafka consumer group consume error: %v", err)
					return
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

	if len(c.subs) == 0 {
		return
	}

	if c.balance {
		idx := atomic.AddUint64(&c.idx, 1) % uint64(len(c.subs))
		handler := c.subs[idx].handler
		if handler != nil {
			task.AddTask(func() { handler(event) })
		}
	} else {
		for _, sub := range c.subs {
			handler := sub.handler
			if handler == nil {
				continue
			}
			task.AddTask(func() { handler(event) })
		}
	}
}

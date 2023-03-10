package kafka

import (
	"context"
	"github.com/Shopify/sarama"
	"github.com/dobyte/due/eventbus"
	"sync"
)

type Eventbus struct {
	ctx    context.Context
	cancel context.CancelFunc
	opts   *options

	err      error
	err1     error
	err2     error
	consumer sarama.Consumer
	producer sarama.AsyncProducer
	builtin  bool

	rw        sync.RWMutex
	consumers map[string]*consumer
}

func NewEventbus(opts ...Option) *Eventbus {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	eb := &Eventbus{}
	eb.opts = o
	eb.consumers = make(map[string]*consumer)
	eb.ctx, eb.cancel = context.WithCancel(o.ctx)

	if o.client != nil {
		eb.consumer, eb.err1 = sarama.NewConsumerFromClient(o.client)
		eb.producer, eb.err2 = sarama.NewAsyncProducerFromClient(o.client)
	} else {
		eb.builtin = true
		config := sarama.NewConfig()
		config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.BalanceStrategyRoundRobin}
		config.Consumer.Return.Errors = true
		config.Producer.Partitioner = sarama.NewHashPartitioner
		config.Producer.RequiredAcks = sarama.WaitForAll
		config.Producer.Return.Successes = true
		config.Producer.Return.Errors = true

		if o.version != "" {
			config.Version, eb.err = sarama.ParseKafkaVersion(o.version)
		}

		if eb.err == nil {
			eb.consumer, eb.err1 = sarama.NewConsumer(o.addrs, config)
			eb.producer, eb.err2 = sarama.NewAsyncProducer(o.addrs, config)
		}
	}

	return eb
}

// Publish 发布事件
func (eb *Eventbus) Publish(ctx context.Context, topic string, payload interface{}) error {
	if eb.err != nil {
		return eb.err
	}

	if eb.err2 != nil {
		return eb.err2
	}

	buf, err := eventbus.PackData(topic, payload)
	if err != nil {
		return err
	}

	eb.producer.Input() <- &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(buf),
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-eb.producer.Successes():
		return nil
	case err = <-eb.producer.Errors():
		return err
	}
}

// Subscribe 订阅事件
func (eb *Eventbus) Subscribe(_ context.Context, topic string, handler eventbus.EventHandler) error {
	if eb.err != nil {
		return eb.err
	}

	if eb.err1 != nil {
		return eb.err1
	}

	eb.rw.Lock()
	c, ok := eb.consumers[topic]
	if !ok {
		c = &consumer{handlers: make(map[uintptr]eventbus.EventHandler)}
		c.ctx, c.cancel = context.WithCancel(eb.ctx)
		eb.consumers[topic] = c
	}
	c.addHandler(handler)
	eb.rw.Unlock()

	if !ok {
		return eb.watch(c, topic)
	}

	return nil
}

// Unsubscribe 取消订阅
func (eb *Eventbus) Unsubscribe(_ context.Context, topic string, handler eventbus.EventHandler) error {
	if eb.err != nil {
		return eb.err
	}

	if eb.err1 != nil {
		return eb.err1
	}

	eb.rw.Lock()
	defer eb.rw.Unlock()

	if c, ok := eb.consumers[topic]; ok {
		if c.remHandler(handler) != 0 {
			return nil
		}
		c.cancel()
		delete(eb.consumers, topic)
	}

	return nil
}

// Close 停止监听
func (eb *Eventbus) Close() error {
	if eb.err != nil {
		return eb.err
	}

	if eb.err1 != nil {
		return eb.err1
	}

	if eb.err2 != nil {
		return eb.err2
	}

	eb.cancel()

	if !eb.builtin {
		return nil
	}

	err1 := eb.consumer.Close()
	err2 := eb.producer.Close()

	if err1 != nil {
		return err1
	}

	return err2
}

func (eb *Eventbus) watch(c *consumer, topic string) error {
	partitions, err := eb.consumer.Partitions(topic)
	if err != nil {
		return err
	}

	for _, partition := range partitions {
		cp, err := eb.consumer.ConsumePartition(topic, partition, sarama.OffsetNewest)
		if err != nil {
			return err
		}

		go func(cp sarama.PartitionConsumer) {
			defer cp.AsyncClose()

			for {
				select {
				case <-c.ctx.Done():
					return
				case message := <-cp.Messages():
					c.dispatch(message.Value)
				case <-cp.Errors():
					return
				}
			}
		}(cp)
	}

	return nil
}

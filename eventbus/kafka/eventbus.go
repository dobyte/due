package kafka

import (
	"context"
	"strings"
	"sync"

	"github.com/IBM/sarama"
	"github.com/dobyte/due/v2/eventbus"
)

type Eventbus struct {
	ctx          context.Context
	cancel       context.CancelFunc
	opts         *options
	err          error
	err1         error
	consumer     sarama.Consumer
	err2         error
	producer     sarama.AsyncProducer
	err3         error
	clusterAdmin sarama.ClusterAdmin
	builtin      bool
	rw           sync.RWMutex
	consumers    map[string]*consumer
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

	if o.client == nil {
		config := sarama.NewConfig()
		config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
		config.Consumer.Return.Errors = true
		config.Producer.Partitioner = sarama.NewHashPartitioner
		config.Producer.RequiredAcks = sarama.WaitForAll
		config.Producer.Return.Successes = true
		config.Producer.Return.Errors = true

		if o.version != "" {
			config.Version, eb.err = sarama.ParseKafkaVersion(o.version)
		}

		if eb.err != nil {
			return eb
		}

		o.client, eb.err = sarama.NewClient(o.addrs, config)

		if eb.err != nil {
			return eb
		}

		eb.builtin = true
	}

	eb.consumer, eb.err1 = sarama.NewConsumerFromClient(o.client)
	eb.producer, eb.err2 = sarama.NewAsyncProducerFromClient(o.client)

	if o.autoCreateTopic {
		eb.clusterAdmin, eb.err3 = sarama.NewClusterAdminFromClient(o.client)
	}

	return eb
}

// Publish 发布事件
func (eb *Eventbus) Publish(ctx context.Context, topic string, payload any) error {
	if eb.err != nil {
		return eb.err
	}

	if eb.err2 != nil {
		return eb.err2
	}

	buf, err := serialize(topic, payload)
	if err != nil {
		return err
	}

	eb.producer.Input() <- &sarama.ProducerMessage{
		Topic: eb.doMakeChannel(topic),
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

	channel := eb.doMakeChannel(topic)

	if eb.opts.autoCreateTopic {
		if eb.err3 != nil {
			return eb.err3
		}

		if err := eb.clusterAdmin.CreateTopic(channel, &sarama.TopicDetail{
			NumPartitions:     1,
			ReplicationFactor: 1,
		}, true); err != nil {
			if e, ok := err.(*sarama.TopicError); ok && e.Err == sarama.ErrTopicAlreadyExists {
				// ignore
			} else {
				return err
			}
		}
	}

	eb.rw.Lock()
	c, ok := eb.consumers[channel]
	if !ok {
		c = &consumer{handlers: make(map[uintptr][]eventbus.EventHandler)}
		c.ctx, c.cancel = context.WithCancel(eb.ctx)
		eb.consumers[channel] = c
	}
	c.addHandler(handler)
	eb.rw.Unlock()

	if !ok {
		return eb.watch(c, channel)
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

	channel := eb.doMakeChannel(topic)

	eb.rw.Lock()
	defer eb.rw.Unlock()

	if c, ok := eb.consumers[channel]; ok {
		if c.delHandler(handler) != 0 {
			return nil
		}
		c.cancel()
		delete(eb.consumers, channel)
	}

	return nil
}

// Close 停止监听
func (eb *Eventbus) Close() error {
	if eb.err != nil {
		return eb.err
	}

	if eb.err1 == nil && eb.consumer != nil {
		_ = eb.consumer.Close()
	}

	if eb.err2 == nil && eb.producer != nil {
		_ = eb.producer.Close()
	}

	if eb.err3 == nil && eb.clusterAdmin != nil {
		_ = eb.clusterAdmin.Close()
	}

	eb.cancel()

	if !eb.builtin {
		return nil
	}

	return eb.opts.client.Close()
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

func (eb *Eventbus) doMakeChannel(topic string) string {
	if eb.opts.prefix == "" {
		return topic
	} else {
		return strings.ReplaceAll(eb.opts.prefix, ":", ".") + "." + topic
	}
}

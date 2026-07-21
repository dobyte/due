package kafka

import (
	"context"
	"strings"
	"sync"

	"github.com/IBM/sarama"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xtime"
	"github.com/dobyte/due/v2/utils/xuuid"
)

type Eventbus struct {
	ctx          context.Context
	cancel       context.CancelFunc
	opts         *options
	err          error
	consumer     sarama.Consumer
	producer     sarama.AsyncProducer
	clusterAdmin sarama.ClusterAdmin
	builtin      bool
	pool         *sync.Pool
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
	eb.pool = &sync.Pool{New: func() any { return &data{} }}
	eb.consumers = make(map[string]*consumer)
	eb.ctx, eb.cancel = context.WithCancel(o.ctx)

	if o.client == nil {
		config := sarama.NewConfig()
		config.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
		config.Consumer.Offsets.Initial = sarama.OffsetNewest
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

	eb.consumer, eb.err = sarama.NewConsumerFromClient(o.client)

	if eb.err != nil {
		return eb
	}

	eb.producer, eb.err = sarama.NewAsyncProducerFromClient(o.client)

	if eb.err != nil {
		return eb
	}

	if o.autoCreateTopic {
		eb.clusterAdmin, eb.err = sarama.NewClusterAdminFromClient(o.client)
	}

	return eb
}

// Publish 发布事件
func (eb *Eventbus) Publish(ctx context.Context, topic string, payload any) error {
	if eb.err != nil {
		return eb.err
	}

	buf, err := eb.serialize(topic, payload)
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
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler, opts ...eventbus.SubscribeOptions) (eventbus.Subscription, error) {
	if eb.err != nil {
		return nil, eb.err
	}

	channel := eb.doMakeChannel(topic)

	eb.rw.Lock()
	defer eb.rw.Unlock()

	single := len(opts) > 0 && opts[0].IsSingleConsumer

	c, ok := eb.consumers[channel]
	if ok {
		if c.single != single {
			return nil, errors.ErrInvalidArgument
		}
	} else {
		if eb.opts.autoCreateTopic && eb.clusterAdmin != nil {
			if err := eb.clusterAdmin.CreateTopic(channel, &sarama.TopicDetail{
				NumPartitions:     1,
				ReplicationFactor: 1,
			}, true); err != nil {
				if e, ok := err.(*sarama.TopicError); ok && e.Err == sarama.ErrTopicAlreadyExists {
					// ignore
				} else {
					return nil, err
				}
			}
		}

		c = newConsumer(eb, single)

		if single {
			groupID := eb.doMakeGroupID(topic)
			group, err := sarama.NewConsumerGroupFromClient(groupID, eb.opts.client)
			if err != nil {
				return nil, err
			}
			c.groupID = groupID
			eb.consumers[channel] = c
			c.startGroupConsumer(group, channel)
		} else {
			eb.consumers[channel] = c
			if err := c.startBroadcastConsumer(eb.consumer, channel); err != nil {
				delete(eb.consumers, channel)
				return nil, err
			}
		}
	}

	sub := c.addSubscription(handler)
	sub.eb = eb
	sub.topic = channel

	return sub, nil
}

// Close 停止监听
func (eb *Eventbus) Close() error {
	if eb.err != nil {
		return eb.err
	}

	eb.rw.Lock()
	for _, c := range eb.consumers {
		c.stop()
		if c.group != nil {
			_ = c.group.Close()
		}
	}
	eb.consumers = make(map[string]*consumer)
	eb.rw.Unlock()

	_ = eb.producer.Close()
	_ = eb.consumer.Close()

	if eb.clusterAdmin != nil {
		_ = eb.clusterAdmin.Close()
	}

	eb.cancel()

	if !eb.builtin {
		return nil
	}

	return eb.opts.client.Close()
}

// 取消订阅
func (eb *Eventbus) unsubscribe(sub *subscription) {
	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[sub.topic]
	if !ok {
		return
	}

	_, empty := c.delSubscription(sub)
	if empty {
		c.stop()
		if c.group != nil {
			_ = c.group.Close()
		}
		delete(eb.consumers, sub.topic)
	}
}

func (eb *Eventbus) doMakeChannel(topic string) string {
	if eb.opts.prefix == "" {
		return topic
	} else {
		return strings.ReplaceAll(eb.opts.prefix, ":", ".") + "." + topic
	}
}

func (eb *Eventbus) doMakeGroupID(topic string) string {
	if eb.opts.prefix == "" {
		return "due:eventbus:queue:" + topic
	} else {
		return strings.ReplaceAll(eb.opts.prefix, ":", ".") + ".queue." + topic
	}
}

func (eb *Eventbus) serialize(topic string, payload any) ([]byte, error) {
	d := eb.pool.Get().(*data)
	defer eb.pool.Put(d)

	d.ID = xuuid.UUID()
	d.Topic = topic
	d.Payload = xconv.String(payload)
	d.Timestamp = xtime.Now().UnixNano()

	return json.Marshal(d)
}

func (eb *Eventbus) deserialize(v []byte) (*eventbus.Event, error) {
	d := eb.pool.Get().(*data)
	defer eb.pool.Put(d)

	if err := json.Unmarshal(v, d); err != nil {
		return nil, err
	}

	return &eventbus.Event{
		ID:        d.ID,
		Topic:     d.Topic,
		Payload:   value.NewValue(d.Payload),
		Timestamp: xtime.UnixNano(d.Timestamp),
	}, nil
}

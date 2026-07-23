package kafka

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/IBM/sarama"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/log"
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
	producer     sarama.SyncProducer
	clusterAdmin sarama.ClusterAdmin
	builtin      bool
	pool         *sync.Pool
	rw           sync.RWMutex
	consumers    map[string]*consumer
	closed       atomic.Bool
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

	eb.producer, eb.err = sarama.NewSyncProducerFromClient(o.client)
	if eb.err != nil {
		return eb
	}

	if o.autoCreateTopic {
		if clusterAdmin, err := sarama.NewClusterAdminFromClient(o.client); err != nil {
			log.Warnf("create cluster admin failed: %v", err)
			o.autoCreateTopic = false
		} else {
			eb.clusterAdmin = clusterAdmin
		}
	}

	return eb
}

// Publish 发布事件
func (eb *Eventbus) Publish(ctx context.Context, topic string, payload any) error {
	if eb.err != nil {
		return eb.err
	}

	if eb.closed.Load() {
		return errors.ErrIllegalOperation
	}

	buf, err := eb.serialize(topic, payload)
	if err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		_, _, err = eb.producer.SendMessage(&sarama.ProducerMessage{
			Topic: eb.doMakeChannel(topic),
			Value: sarama.ByteEncoder(buf),
		})
		return err
	}
}

// Subscribe 订阅事件
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler, balance ...bool) (eventbus.Subscription, error) {
	if eb.err != nil {
		return nil, eb.err
	}

	if eb.closed.Load() {
		return nil, errors.ErrIllegalOperation
	}

	channel := eb.doMakeChannel(topic)
	lb := len(balance) > 0 && balance[0]

	eb.rw.RLock()
	c, ok := eb.consumers[channel]
	eb.rw.RUnlock()

	if ok {
		if c.balance != lb {
			return nil, errors.ErrInvalidArgument
		}

		sub := c.addSubscription(handler)
		sub.eb = eb
		sub.topic = channel

		return sub, nil
	}

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

	c = newConsumer(eb, lb)

	if lb {
		groupID := eb.doMakeGroupID(topic)
		group, err := sarama.NewConsumerGroupFromClient(groupID, eb.opts.client)
		if err != nil {
			return nil, err
		}
		c.groupID = groupID
		c.group = group
	}

	eb.rw.Lock()

	if eb.closed.Load() {
		eb.rw.Unlock()

		if c.group != nil {
			_ = c.group.Close()
		}
		return nil, errors.ErrIllegalOperation
	}

	if existing, ok := eb.consumers[channel]; ok {
		eb.rw.Unlock()

		if existing.balance != lb {
			if c.group != nil {
				_ = c.group.Close()
			}
			return nil, errors.ErrInvalidArgument
		}

		sub := existing.addSubscription(handler)
		sub.eb = eb
		sub.topic = channel

		if c.group != nil {
			_ = c.group.Close()
		}

		return sub, nil
	}

	eb.consumers[channel] = c

	if lb {
		c.startGroupConsumer(c.group, channel)
	} else {
		if err := c.startBroadcastConsumer(eb.consumer, channel); err != nil {
			delete(eb.consumers, channel)
			eb.rw.Unlock()

			if c.group != nil {
				_ = c.group.Close()
			}
			return nil, err
		}
	}

	eb.rw.Unlock()

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

	if !eb.closed.CompareAndSwap(false, true) {
		return nil
	}

	eb.cancel()

	eb.rw.Lock()
	consumers := make([]*consumer, 0, len(eb.consumers))
	for _, c := range eb.consumers {
		consumers = append(consumers, c)
	}
	eb.consumers = make(map[string]*consumer)
	eb.rw.Unlock()

	for _, c := range consumers {
		c.stop()
		if c.group != nil {
			_ = c.group.Close()
		}
	}

	_ = eb.producer.Close()
	_ = eb.consumer.Close()

	if eb.clusterAdmin != nil {
		_ = eb.clusterAdmin.Close()
	}

	if !eb.builtin {
		return nil
	}

	return eb.opts.client.Close()
}

// 取消订阅
func (eb *Eventbus) unsubscribe(sub *subscription) {
	eb.rw.Lock()

	c, ok := eb.consumers[sub.topic]
	if !ok {
		eb.rw.Unlock()
		return
	}

	_, empty := c.delSubscription(sub)
	if empty {
		delete(eb.consumers, sub.topic)
		eb.rw.Unlock()

		c.stop()
		if c.group != nil {
			_ = c.group.Close()
		}
	} else {
		eb.rw.Unlock()
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

	event := &eventbus.Event{
		ID:        d.ID,
		Topic:     d.Topic,
		Payload:   value.NewValue(d.Payload),
		Timestamp: xtime.UnixNano(d.Timestamp),
	}

	d.ID = ""
	d.Topic = ""
	d.Payload = ""
	d.Timestamp = 0

	return event, nil
}

package redis

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xtime"
	"github.com/dobyte/due/v2/utils/xuuid"
	"github.com/redis/go-redis/v9"
)

type Eventbus struct {
	err          error
	ctx          context.Context
	cancel       context.CancelFunc
	builtin      bool
	opts         *options
	pool         *sync.Pool
	sub          *redis.PubSub
	wg           sync.WaitGroup
	rw           sync.RWMutex
	consumers    map[string]*consumer
	groupSubs    map[string][]*subscription
	groupCancels map[string]context.CancelFunc
}

func NewEventbus(opts ...Option) *Eventbus {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	eb := &Eventbus{}

	defer func() {
		if eb.err == nil {
			eb.opts = o
			eb.pool = &sync.Pool{New: func() any { return &data{} }}
			eb.ctx, eb.cancel = context.WithCancel(o.ctx)
			eb.sub = eb.opts.client.Subscribe(eb.ctx)
			eb.consumers = make(map[string]*consumer)
			eb.groupSubs = make(map[string][]*subscription)
			eb.groupCancels = make(map[string]context.CancelFunc)

			go eb.watch()
		}
	}()

	if o.client == nil {
		options := &redis.UniversalOptions{
			Addrs:      o.addrs,
			DB:         o.db,
			Username:   o.username,
			Password:   o.password,
			MaxRetries: o.maxRetries,
		}

		if o.certFile != "" && o.keyFile != "" && o.caFile != "" {
			if options.TLSConfig, eb.err = tls.MakeRedisTLSConfig(o.certFile, o.keyFile, o.caFile); eb.err != nil {
				return eb
			}
		}

		o.client, eb.builtin = redis.NewUniversalClient(options), true
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

	channel := eb.doMakeChannel(topic)
	stream := eb.doMakeStream(topic)

	if err = eb.opts.client.Publish(ctx, channel, buf).Err(); err != nil {
		return err
	}

	return eb.opts.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]any{"payload": xconv.String(buf)},
	}).Err()
}

// Subscribe 订阅事件
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler, opts ...eventbus.SubscribeOptions) (eventbus.Subscription, error) {
	if eb.err != nil {
		return nil, eb.err
	}

	single := len(opts) > 0 && opts[0].IsSingleConsumer

	if single {
		return eb.subscribeGroup(ctx, topic, handler)
	}

	return eb.subscribeBroadcast(ctx, topic, handler)
}

func (eb *Eventbus) subscribeBroadcast(ctx context.Context, topic string, handler eventbus.EventHandler) (eventbus.Subscription, error) {
	channel := eb.doMakeChannel(topic)

	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[channel]
	if !ok {
		if err := eb.sub.Subscribe(ctx, channel); err != nil {
			return nil, err
		}
		c = newConsumer(eb)
		eb.consumers[channel] = c
	}

	sub := c.addSubscription(handler)
	sub.eb = eb
	sub.topic = channel

	return sub, nil
}

func (eb *Eventbus) subscribeGroup(ctx context.Context, topic string, handler eventbus.EventHandler) (eventbus.Subscription, error) {
	stream := eb.doMakeStream(topic)
	group := eb.doMakeGroupID(topic)

	eb.rw.Lock()
	subs, ok := eb.groupSubs[stream]
	if ok && len(subs) > 0 {
		sub := &subscription{
			eb:      eb,
			topic:   stream,
			handler: handler,
			single:  true,
			stream:  stream,
			group:   group,
		}
		subs = append(subs, sub)
		eb.groupSubs[stream] = subs
		eb.rw.Unlock()
		return sub, nil
	}

	_, err := eb.opts.client.XGroupCreateMkStream(ctx, stream, group, "$").Result()
	if err != nil && !isBusyGroupError(err) {
		eb.rw.Unlock()
		return nil, err
	}

	sub := &subscription{
		eb:      eb,
		topic:   stream,
		handler: handler,
		single:  true,
		stream:  stream,
		group:   group,
	}

	subs = append(subs, sub)
	eb.groupSubs[stream] = subs
	ctx, cancel := context.WithCancel(eb.ctx)
	eb.groupCancels[stream] = cancel
	eb.wg.Add(1)
	eb.rw.Unlock()

	go eb.watchGroup(ctx, stream, group)

	return sub, nil
}

// Close 停止监听
func (eb *Eventbus) Close() error {
	if eb.err != nil {
		return eb.err
	}

	eb.cancel()

	_ = eb.sub.Close()

	eb.wg.Wait()

	if eb.builtin {
		return eb.opts.client.Close()
	}

	return nil
}

// 取消订阅
func (eb *Eventbus) unsubscribe(ctx context.Context, sub *subscription) {
	if sub.single {
		eb.unsubscribeGroup(ctx, sub)
	} else {
		eb.unsubscribeBroadcast(ctx, sub.topic, sub)
	}
}

func (eb *Eventbus) unsubscribeBroadcast(ctx context.Context, topic string, sub *subscription) {
	eb.rw.Lock()
	defer eb.rw.Unlock()

	c, ok := eb.consumers[topic]
	if !ok {
		return
	}

	if _, empty := c.delSubscription(sub); empty {
		_ = eb.sub.Unsubscribe(ctx, topic)
		delete(eb.consumers, topic)
	}
}

func (eb *Eventbus) unsubscribeGroup(_ context.Context, sub *subscription) {
	eb.rw.Lock()
	defer eb.rw.Unlock()

	subs, ok := eb.groupSubs[sub.stream]
	if !ok {
		return
	}

	result := make([]*subscription, 0, len(subs))
	for _, s := range subs {
		if s != sub {
			result = append(result, s)
		}
	}

	if len(result) == 0 {
		delete(eb.groupSubs, sub.stream)
		if cancel, ok := eb.groupCancels[sub.stream]; ok {
			cancel()
			delete(eb.groupCancels, sub.stream)
		}
	} else {
		eb.groupSubs[sub.stream] = result
	}
}

// watch 监听广播事件
func (eb *Eventbus) watch() {
	for {
		iface, err := eb.sub.Receive(eb.ctx)
		if err != nil {
			return
		}

		switch v := iface.(type) {
		case *redis.Message:
			eb.rw.RLock()
			c, ok := eb.consumers[v.Channel]
			eb.rw.RUnlock()
			if ok {
				c.dispatch(xconv.Bytes(v.Payload))
			}
		}
	}
}

// watchGroup 监听消费组事件
func (eb *Eventbus) watchGroup(ctx context.Context, stream, group string) {
	defer eb.wg.Done()

	consumer := group + "-" + xuuid.UUID()
	var index uint64

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		streams, err := eb.opts.client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    group,
			Consumer: consumer,
			Streams:  []string{stream, ">"},
			Count:    1,
			Block:    5 * time.Second,
		}).Result()
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, redis.Nil) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				continue
			}
			log.Errorf("read stream failed: %v", err)
			time.Sleep(time.Second)
			continue
		}

		for _, s := range streams {
			for _, msg := range s.Messages {
				payload, ok := msg.Values["payload"].(string)
				if !ok {
					continue
				}

				event, err := eb.deserialize(xconv.Bytes(payload))
				if err != nil {
					log.Errorf("invalid event data: %v", err)
					continue
				}

				eb.rw.RLock()
				subs, ok := eb.groupSubs[stream]
				if ok && len(subs) > 0 {
					idx := index % uint64(len(subs))
					sub := subs[idx]
					index++
					handler := sub.handler
					if handler != nil {
						task.AddTask(func() { handler(event) })
					}
				}
				eb.rw.RUnlock()

				if _, err := eb.opts.client.XAck(ctx, stream, group, msg.ID).Result(); err != nil {
					if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
						log.Errorf("ack stream message failed: %v", err)
					}
				}
			}
		}
	}
}

func (eb *Eventbus) doMakeChannel(topic string) string {
	if eb.opts.prefix == "" {
		return topic
	} else {
		return eb.opts.prefix + ":" + topic
	}
}

func (eb *Eventbus) doMakeStream(topic string) string {
	if eb.opts.prefix == "" {
		return "due:eventbus:stream:" + topic
	} else {
		return eb.opts.prefix + ":stream:" + topic
	}
}

func (eb *Eventbus) doMakeGroupID(topic string) string {
	if eb.opts.prefix == "" {
		return "due:eventbus:queue:" + topic
	} else {
		return eb.opts.prefix + ":queue:" + topic
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

func isBusyGroupError(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}

package redis

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/dobyte/due/v2/core/tls"
	"github.com/dobyte/due/v2/core/value"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xuuid"
	"github.com/redis/go-redis/v9"
)

// minIDClockBuffer is the clock skew buffer.
// MINID trimming is computed from the publisher's local clock, whereas stream entry IDs use the
// Redis server clock. The buffer absorbs the skew between them so that a freshly written message
// is not trimmed immediately.
const minIDClockBuffer = time.Second

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

// Publish publishes an event.
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

	// Set the message retention window: messages older than it are trimmed and dropped on write.
	// When staleDuration is 0, messages are not retained and unconsumed messages are dropped immediately.
	xaddArgs := &redis.XAddArgs{
		Stream: stream,
		MinID:  strconv.FormatInt(time.Now().Add(-eb.opts.staleDuration).Add(-minIDClockBuffer).UnixMilli(), 10),
		Values: map[string]any{"payload": xconv.String(buf)},
	}

	// Persist to the stream first, then broadcast to the channel; even if the broadcast fails, the
	// message remains in the stream and can still be consumed.
	if err = eb.opts.client.XAdd(ctx, xaddArgs).Err(); err != nil {
		return err
	}

	return eb.opts.client.Publish(ctx, channel, buf).Err()
}

// Subscribe subscribes to an event.
func (eb *Eventbus) Subscribe(ctx context.Context, topic string, handler eventbus.EventHandler, balance ...bool) (eventbus.Subscription, error) {
	if eb.err != nil {
		return nil, eb.err
	}

	if len(balance) > 0 && balance[0] {
		return eb.subscribeGroup(ctx, topic, handler)
	} else {
		return eb.subscribeBroadcast(ctx, topic, handler)
	}
}

func (eb *Eventbus) subscribeBroadcast(ctx context.Context, topic string, handler eventbus.EventHandler) (eventbus.Subscription, error) {
	channel := eb.doMakeChannel(topic)

	// Fast path: reuse the existing consumer to avoid duplicate subscriptions.
	eb.rw.Lock()
	if c, ok := eb.consumers[channel]; ok {
		sub := c.addSubscription(handler)
		sub.eb = eb
		sub.topic = channel
		eb.rw.Unlock()
		return sub, nil
	}
	eb.rw.Unlock()

	// Slow path: perform the network I/O outside the lock to avoid blocking other subscribe
	// operations.
	if err := eb.sub.Subscribe(ctx, channel); err != nil {
		return nil, err
	}

	// Reacquire the lock to finish registration, avoiding concurrent duplicate consumer creation.
	eb.rw.Lock()
	defer eb.rw.Unlock()

	if c, ok := eb.consumers[channel]; ok {
		sub := c.addSubscription(handler)
		sub.eb = eb
		sub.topic = channel
		return sub, nil
	}

	c := newConsumer(eb)
	eb.consumers[channel] = c

	sub := c.addSubscription(handler)
	sub.eb = eb
	sub.topic = channel

	return sub, nil
}

func (eb *Eventbus) subscribeGroup(ctx context.Context, topic string, handler eventbus.EventHandler) (eventbus.Subscription, error) {
	stream := eb.doMakeStream(topic)
	group := eb.doMakeGroupID(topic)

	sub := &subscription{
		eb:      eb,
		topic:   stream,
		handler: handler,
		single:  true,
		stream:  stream,
		group:   group,
	}

	// Fast path: reuse the existing subscriber to avoid creating duplicate consumer groups.
	eb.rw.Lock()
	if subs, ok := eb.groupSubs[stream]; ok && len(subs) > 0 {
		eb.groupSubs[stream] = append(subs, sub)
		eb.rw.Unlock()
		return sub, nil
	}
	eb.rw.Unlock()

	// Slow path: perform the network I/O outside the lock to avoid blocking other subscribe
	// operations.
	if _, err := eb.opts.client.XGroupCreateMkStream(ctx, stream, group, "$").Result(); err != nil && !isBusyGroupError(err) {
		return nil, err
	}

	// Reacquire the lock to finish registration, avoiding concurrent duplicate consumer creation.
	eb.rw.Lock()
	defer eb.rw.Unlock()

	if subs, ok := eb.groupSubs[stream]; ok && len(subs) > 0 {
		eb.groupSubs[stream] = append(subs, sub)
		return sub, nil
	}

	eb.groupSubs[stream] = append(eb.groupSubs[stream], sub)

	watchCtx, cancel := context.WithCancel(eb.ctx)
	eb.groupCancels[stream] = cancel
	eb.wg.Add(1)

	go eb.watchGroup(watchCtx, stream, group)

	return sub, nil
}

// Close stops listening.
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

// unsubscribe cancels a subscription.
func (eb *Eventbus) unsubscribe(ctx context.Context, sub *subscription) {
	if sub.single {
		eb.unsubscribeGroup(ctx, sub)
	} else {
		eb.unsubscribeBroadcast(ctx, sub.topic, sub)
	}
}

func (eb *Eventbus) unsubscribeBroadcast(ctx context.Context, topic string, sub *subscription) {
	eb.rw.Lock()

	c, ok := eb.consumers[topic]
	if !ok {
		eb.rw.Unlock()
		return
	}

	_, empty := c.delSubscription(sub)
	if empty {
		delete(eb.consumers, topic)
	}
	eb.rw.Unlock()

	// Perform the unsubscribe network I/O outside the lock, only when the last subscription is removed.
	if empty {
		_ = eb.sub.Unsubscribe(ctx, topic)
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

// watch listens for broadcast events.
func (eb *Eventbus) watch() {
	backoff := 100 * time.Millisecond
	maxBackoff := 10 * time.Second

	for {
		iface, err := eb.sub.Receive(eb.ctx)
		if err != nil {
			if eb.ctx.Err() != nil {
				return
			}
			log.Errorf("receive pubsub message failed: %v", err)
			time.Sleep(backoff)
			if backoff < maxBackoff {
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
			}
			continue
		}

		backoff = 100 * time.Millisecond

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

// watchGroup listens for consumer group events.
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

		// Reclaim pending messages that were not acknowledged due to network jitter and similar
		// issues, so that they do not get stuck for a long time.
		eb.reclaimPending(ctx, stream, group, consumer, &index)

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
				eb.process(ctx, stream, group, msg, &index)
			}
		}
	}
}

// reclaimPending reclaims messages that have been idle for too long in the pending list.
func (eb *Eventbus) reclaimPending(ctx context.Context, stream, group, consumer string, index *uint64) {
	messages, _, err := eb.opts.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   stream,
		Group:    group,
		Consumer: consumer,
		MinIdle:  5 * time.Second,
		Start:    "0-0",
		Count:    10,
	}).Result()
	if err != nil {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, redis.Nil) {
			log.Errorf("reclaim pending message failed: %v", err)
		}
		return
	}

	for _, msg := range messages {
		eb.process(ctx, stream, group, msg, index)
	}
}

// process handles a single stream message.
func (eb *Eventbus) process(ctx context.Context, stream, group string, msg redis.XMessage, index *uint64) {
	payload, ok := msg.Values["payload"].(string)
	if !ok {
		return
	}

	event, err := eb.deserialize(xconv.Bytes(payload))
	if err != nil {
		log.Errorf("invalid event data: %v", err)
		return
	}

	// The event is stale; acknowledge and drop it directly to avoid consuming stale messages.
	if eb.opts.staleDuration > 0 && time.Since(event.Timestamp) > eb.opts.staleDuration {
		if _, err := eb.opts.client.XAck(ctx, stream, group, msg.ID).Result(); err != nil {
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				log.Errorf("ack expired stream message failed: %v", err)
			}
		}
		return
	}

	eb.rw.RLock()
	subs, ok := eb.groupSubs[stream]
	if ok && len(subs) > 0 {
		idx := *index % uint64(len(subs))
		sub := subs[idx]
		(*index)++
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

// serialize serializes an event.
func (eb *Eventbus) serialize(topic string, payload any) ([]byte, error) {
	d := eb.pool.Get().(*data)
	defer eb.pool.Put(d)

	d.ID = xuuid.UUID()
	d.Topic = topic
	d.Payload = xconv.String(payload)
	d.Timestamp = time.Now().UnixNano()

	return json.Marshal(d)
}

// deserialize deserializes an event.
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
		Timestamp: time.Unix(d.Timestamp/1e9, d.Timestamp%1e9),
	}, nil
}

func isBusyGroupError(err error) bool {
	return err != nil && err.Error() == "BUSYGROUP Consumer Group name already exists"
}

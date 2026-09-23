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

// minIDClockBuffer 时钟偏差缓冲
// MINID裁剪基于发布方本地时钟计算，而Stream条目ID使用Redis服务器时钟，
// 需要预留缓冲以吸收两者之间的时钟偏差，避免刚写入的消息被立即裁剪
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

	// 设置消息保留时长：超过该时长的消息会在写入时被自动裁剪丢弃
	// staleDuration为0时消息不保留，未被消费的消息将被立即丢弃
	xaddArgs := &redis.XAddArgs{
		Stream: stream,
		MinID:  strconv.FormatInt(time.Now().Add(-eb.opts.staleDuration).Add(-minIDClockBuffer).UnixMilli(), 10),
		Values: map[string]any{"payload": xconv.String(buf)},
	}

	// 先写入Stream持久化，再广播到频道；即使广播失败，消息仍保留在Stream中可被消费
	if err = eb.opts.client.XAdd(ctx, xaddArgs).Err(); err != nil {
		return err
	}

	return eb.opts.client.Publish(ctx, channel, buf).Err()
}

// Subscribe 订阅事件
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

	// 快路径：消费者已存在时直接复用，避免重复订阅
	eb.rw.Lock()
	if c, ok := eb.consumers[channel]; ok {
		sub := c.addSubscription(handler)
		sub.eb = eb
		sub.topic = channel
		eb.rw.Unlock()
		return sub, nil
	}
	eb.rw.Unlock()

	// 慢路径：将网络 I/O 移出锁外，避免长时间持锁阻塞其他订阅操作
	if err := eb.sub.Subscribe(ctx, channel); err != nil {
		return nil, err
	}

	// 重新加锁完成登记，避免并发重复创建消费者
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

	// 快路径：已存在订阅者时直接复用，避免重复创建消费组
	eb.rw.Lock()
	if subs, ok := eb.groupSubs[stream]; ok && len(subs) > 0 {
		eb.groupSubs[stream] = append(subs, sub)
		eb.rw.Unlock()
		return sub, nil
	}
	eb.rw.Unlock()

	// 慢路径：将网络 I/O 移出锁外，避免长时间持锁阻塞其他订阅操作
	if _, err := eb.opts.client.XGroupCreateMkStream(ctx, stream, group, "$").Result(); err != nil && !isBusyGroupError(err) {
		return nil, err
	}

	// 重新加锁完成登记，避免并发重复创建消费者
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

	// 将退订网络 I/O 移出锁外，仅在最后一个订阅被移除时执行
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

// watch 监听广播事件
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

		// 回收因网络抖动等原因未能确认的pending消息，避免消息长期卡死
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

// reclaimPending 回收pending列表中闲置过久的消息
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

// process 处理单条流消息
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

	// 事件已过期，直接确认并丢弃，避免消费过期消息
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

// 序列化事件
func (eb *Eventbus) serialize(topic string, payload any) ([]byte, error) {
	d := eb.pool.Get().(*data)
	defer eb.pool.Put(d)

	d.ID = xuuid.UUID()
	d.Topic = topic
	d.Payload = xconv.String(payload)
	d.Timestamp = time.Now().UnixNano()

	return json.Marshal(d)
}

// 反序列化事件
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

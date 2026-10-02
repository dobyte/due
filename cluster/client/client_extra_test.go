package client

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

// tAttr is a connection attribute implementation for tests.
type tAttr struct {
	m map[any]any
}

func (a *tAttr) Set(key, value any)      { a.m[key] = value }
func (a *tAttr) Get(key any) (any, bool) { v, ok := a.m[key]; return v, ok }
func (a *tAttr) Del(key any) bool        { _, ok := a.m[key]; delete(a.m, key); return ok }
func (a *tAttr) Clear()                  { a.m = make(map[any]any) }
func (a *tAttr) Visit(fn func(key, value any) bool) {
	for k, v := range a.m {
		if !fn(k, v) {
			return
		}
	}
}

// tConn is a network connection implementation for tests.
type tConn struct {
	id      int64
	uid     int64
	attr    *tAttr
	pushErr error
	pushed  chan buffer.Buffer
	closed  atomic.Bool
}

func newTConn(id int64) *tConn {
	return &tConn{id: id, attr: &tAttr{m: make(map[any]any)}, pushed: make(chan buffer.Buffer, 8)}
}

func (c *tConn) ID() int64                     { return c.id }
func (c *tConn) UID() int64                    { return c.uid }
func (c *tConn) Attr() network.Attr            { return c.attr }
func (c *tConn) Bind(uid int64) error          { c.uid = uid; return nil }
func (c *tConn) Unbind() error                 { c.uid = 0; return nil }
func (c *tConn) State() network.ConnState      { return network.ConnOpened }
func (c *tConn) Close(force ...bool) error     { c.closed.Store(true); return nil }
func (c *tConn) LocalIP() (string, error)      { return "127.0.0.1", nil }
func (c *tConn) LocalAddr() (net.Addr, error)  { return nil, nil }
func (c *tConn) RemoteIP() (string, error)     { return "10.0.0.1", nil }
func (c *tConn) RemoteAddr() (net.Addr, error) { return nil, nil }

func (c *tConn) Push(buf buffer.Buffer) error {
	if c.pushErr != nil {
		return c.pushErr
	}

	select {
	case c.pushed <- buf:
	default:
		buf.Release()
	}

	return nil
}

// tNetwork is a network client implementation for tests.
type tNetwork struct {
	conn      network.Conn
	dialErr   error
	dialCount atomic.Int32

	connect    network.ConnectHandler
	receive    network.ReceiveHandler
	disconnect network.DisconnectHandler
	heartbeat  network.HeartbeatHandler
}

func (c *tNetwork) Dial(addr ...string) (network.Conn, error) {
	if c.dialErr != nil {
		return nil, c.dialErr
	}
	c.dialCount.Add(1)
	return c.conn, nil
}

func (c *tNetwork) Protocol() string                               { return "test" }
func (c *tNetwork) OnConnect(handler network.ConnectHandler)       { c.connect = handler }
func (c *tNetwork) OnHeartbeat(handler network.HeartbeatHandler)   { c.heartbeat = handler }
func (c *tNetwork) OnReceive(handler network.ReceiveHandler)       { c.receive = handler }
func (c *tNetwork) OnDisconnect(handler network.DisconnectHandler) { c.disconnect = handler }

// tEncryptor is an encryptor implementation for tests.
type tEncryptor struct {
	encryptErr error
	decryptErr error
}

func (e *tEncryptor) Name() string { return "test-encryptor" }

func (e *tEncryptor) Encrypt(data []byte) ([]byte, error) {
	if e.encryptErr != nil {
		return nil, e.encryptErr
	}
	return append([]byte(nil), data...), nil
}

func (e *tEncryptor) Decrypt(data []byte) ([]byte, error) {
	if e.decryptErr != nil {
		return nil, e.decryptErr
	}
	return append([]byte(nil), data...), nil
}

// tCodec is a codec implementation whose marshal and unmarshal always fail.
type tCodec struct{}

func (tCodec) Name() string                       { return "failing" }
func (tCodec) Marshal(v any) ([]byte, error)      { return nil, errors.New("marshal failed") }
func (tCodec) Unmarshal(data []byte, v any) error { return errors.New("unmarshal failed") }

// clientFixture bundles a client and the test doubles it depends on.
type clientFixture struct {
	client  *Client
	network *tNetwork
	conn    *tConn
}

func newClientFixture(t *testing.T) *clientFixture {
	t.Helper()

	nc := newTConn(1)
	net := &tNetwork{conn: nc}

	c := NewClient(
		WithID("test-client-id"),
		WithName("test-client-name"),
		WithClient(net),
		WithCodec(json.DefaultCodec),
	)

	return &clientFixture{client: c, network: net, conn: nc}
}

// start initializes and starts the client, ensuring it is torn down when the test ends.
func (f *clientFixture) start(t *testing.T) {
	t.Helper()

	f.client.Init()
	f.client.Start()

	t.Cleanup(func() {
		f.client.Close()
		f.client.conns.Clear()
		f.client.Destroy()
	})
}

func TestClientNewAndName(t *testing.T) {
	c := NewClient(WithName("my-client"))

	if c.Name() != "my-client" {
		t.Fatalf("unexpected name: %s", c.Name())
	}
	if c.getState() != cluster.Shut {
		t.Fatalf("a new client must be shut, got %v", c.getState())
	}
	if c.proxy == nil {
		t.Fatal("client proxy must be initialized")
	}
}

func TestClientDefaultOptions(t *testing.T) {
	o := defaultOptions()

	if o.name != defaultName {
		t.Fatalf("unexpected default name: %s", o.name)
	}
	if o.codec == nil {
		t.Fatal("default codec must not be nil")
	}
	if o.id == "" {
		t.Fatal("default id must not be empty")
	}
}

func TestClientOptions(t *testing.T) {
	tests := []struct {
		name  string
		apply func(o *options)
		check func(t *testing.T, o *options)
	}{
		{
			name:  "WithID valid",
			apply: func(o *options) { WithID("id-1")(o) },
			check: func(t *testing.T, o *options) {
				if o.id != "id-1" {
					t.Fatalf("unexpected id: %s", o.id)
				}
			},
		},
		{
			name:  "WithID empty",
			apply: func(o *options) { WithID("")(o) },
			check: func(t *testing.T, o *options) {
				if o.id == "" {
					t.Fatal("empty id must be ignored")
				}
			},
		},
		{
			name:  "WithName valid",
			apply: func(o *options) { WithName("n")(o) },
			check: func(t *testing.T, o *options) {
				if o.name != "n" {
					t.Fatalf("unexpected name: %s", o.name)
				}
			},
		},
		{
			name:  "WithName empty",
			apply: func(o *options) { WithName("")(o) },
			check: func(t *testing.T, o *options) {
				if o.name != defaultName {
					t.Fatalf("empty name must be ignored, got %s", o.name)
				}
			},
		},
		{
			name:  "WithCodec valid",
			apply: func(o *options) { WithCodec(json.DefaultCodec)(o) },
			check: func(t *testing.T, o *options) {
				if o.codec == nil {
					t.Fatal("codec must be set")
				}
			},
		},
		{
			name:  "WithCodec nil",
			apply: func(o *options) { o.codec = nil; WithCodec(nil)(o) },
			check: func(t *testing.T, o *options) {
				if o.codec != nil {
					t.Fatal("nil codec must be ignored")
				}
			},
		},
		{
			name:  "WithClient valid",
			apply: func(o *options) { WithClient(&tNetwork{})(o) },
			check: func(t *testing.T, o *options) {
				if o.client == nil {
					t.Fatal("client must be set")
				}
			},
		},
		{
			name:  "WithClient nil",
			apply: func(o *options) { WithClient(nil)(o) },
			check: func(t *testing.T, o *options) {
				if o.client != nil {
					t.Fatal("nil client must be ignored")
				}
			},
		},
		{
			name:  "WithContext valid",
			apply: func(o *options) { WithContext(context.Background())(o) },
			check: func(t *testing.T, o *options) {
				if o.ctx == nil {
					t.Fatal("ctx must be set")
				}
			},
		},
		{
			name:  "WithContext nil",
			apply: func(o *options) { WithContext(nil)(o) },
			check: func(t *testing.T, o *options) {
				if o.ctx == nil {
					t.Fatal("nil ctx must be ignored")
				}
			},
		},
		{
			name:  "WithEncryptor valid",
			apply: func(o *options) { WithEncryptor(&tEncryptor{})(o) },
			check: func(t *testing.T, o *options) {
				if o.encryptor == nil {
					t.Fatal("encryptor must be set")
				}
			},
		},
		{
			name:  "WithEncryptor nil",
			apply: func(o *options) { WithEncryptor(nil)(o) },
			check: func(t *testing.T, o *options) {
				if o.encryptor != nil {
					t.Fatal("nil encryptor must be ignored")
				}
			},
		},
		{
			name: "WithDialAddr",
			apply: func(o *options) {
				do := &dialOptions{attrs: make(map[string]any)}
				WithDialAddr("127.0.0.1:9000")(do)
				o.ctx = context.WithValue(o.ctx, "addr", do.addr)
			},
			check: func(t *testing.T, o *options) {
				if o.ctx.Value("addr") != "127.0.0.1:9000" {
					t.Fatalf("unexpected dial addr: %v", o.ctx.Value("addr"))
				}
			},
		},
		{
			name: "WithConnAttr",
			apply: func(o *options) {
				do := &dialOptions{attrs: make(map[string]any)}
				WithConnAttr("k", "v")(do)
				o.ctx = context.WithValue(o.ctx, "attr", do.attrs["k"])
			},
			check: func(t *testing.T, o *options) {
				if o.ctx.Value("attr") != "v" {
					t.Fatalf("unexpected conn attr: %v", o.ctx.Value("attr"))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := defaultOptions()
			tt.apply(o)
			tt.check(t, o)
		})
	}
}

func TestClientLifecycle(t *testing.T) {
	f := newClientFixture(t)

	var inits, starts, closes, destroys atomic.Int32
	f.client.Proxy().AddHookListener(cluster.Init, func(p *Proxy) { inits.Add(1) })
	f.client.Proxy().AddHookListener(cluster.Start, func(p *Proxy) { starts.Add(1) })
	f.client.Proxy().AddHookListener(cluster.Close, func(p *Proxy) { closes.Add(1) })
	f.client.Proxy().AddHookListener(cluster.Destroy, func(p *Proxy) { destroys.Add(1) })

	f.client.Init()
	f.client.Start()

	if inits.Load() != 1 || starts.Load() != 1 {
		t.Fatalf("unexpected hook counts: init=%d start=%d", inits.Load(), starts.Load())
	}
	if f.client.getState() != cluster.Work {
		t.Fatalf("client must be working, got %v", f.client.getState())
	}
	if f.network.disconnect == nil || f.network.receive == nil {
		t.Fatal("network handlers must be registered")
	}

	// Starting again must be a no-op.
	f.client.Start()

	f.client.Close()
	if f.client.getState() != cluster.Hang {
		t.Fatalf("client must be hanging, got %v", f.client.getState())
	}
	if closes.Load() != 1 {
		t.Fatalf("unexpected close count: %d", closes.Load())
	}

	f.client.Destroy()
	if f.client.getState() != cluster.Shut {
		t.Fatalf("client must be shut, got %v", f.client.getState())
	}
	if destroys.Load() != 1 {
		t.Fatalf("unexpected destroy count: %d", destroys.Load())
	}
}

func TestClientCloseFromBusy(t *testing.T) {
	f := newClientFixture(t)

	f.client.state.Store(int32(cluster.Busy))
	f.client.Close()

	if f.client.getState() != cluster.Hang {
		t.Fatalf("client must be hanging, got %v", f.client.getState())
	}
}

func TestClientDestroyClosesConns(t *testing.T) {
	f := newClientFixture(t)

	f.client.Init()
	f.client.Start()

	cc := &Conn{conn: f.conn, client: f.client}
	f.client.conns.Store(f.conn, cc)

	f.client.Close()
	f.client.Destroy()

	if f.client.getState() != cluster.Shut {
		t.Fatalf("client must be shut, got %v", f.client.getState())
	}
	if !f.conn.closed.Load() {
		t.Fatal("conn must be closed on destroy")
	}
	if _, ok := f.client.conns.Load(f.conn); ok {
		t.Fatal("connection table must be cleared on destroy")
	}
}

func TestClientDial(t *testing.T) {
	f := newClientFixture(t)
	f.start(t)

	// Dialing while shut is rejected.
	shut := NewClient(WithClient(&tNetwork{conn: newTConn(9)}), WithCodec(json.DefaultCodec))
	if _, err := shut.Proxy().Dial(); !errors.Is(err, errors.ErrClientShut) {
		t.Fatalf("expect ErrClientShut, got %v", err)
	}

	cc, err := f.client.Proxy().Dial(WithDialAddr("127.0.0.1:9000"), WithConnAttr("k", "v"))
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	if cc.ID() != f.conn.ID() {
		t.Fatalf("unexpected conn id: %d", cc.ID())
	}
	if got := cc.GetAttr("k").String(); got != "v" {
		t.Fatalf("conn attr not applied: %s", got)
	}

	// A dial failure is propagated.
	f.network.dialErr = errors.New("dial failed")
	if _, err = f.client.Proxy().Dial(); err == nil {
		t.Fatal("expect a dial error")
	}
}

func TestClientDialTriggersConnectEvent(t *testing.T) {
	f := newClientFixture(t)

	ch := make(chan *Conn, 1)
	f.client.Proxy().AddEventListener(cluster.Connect, func(conn *Conn) { ch <- conn })
	f.start(t)

	if _, err := f.client.Proxy().Dial(); err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	select {
	case conn := <-ch:
		if conn.ID() != f.conn.ID() {
			t.Fatalf("unexpected conn: %d", conn.ID())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("connect event not received")
	}
}

func TestClientHandleReceiveRoute(t *testing.T) {
	f := newClientFixture(t)

	result := make(chan int, 1)
	f.client.Proxy().AddRouteHandler(1, func(ctx *Context) {
		var out struct {
			N int `json:"n"`
		}
		if err := ctx.Parse(&out); err != nil {
			result <- -1
			return
		}
		result <- out.N
	})
	f.start(t)

	cc := &Conn{conn: f.conn, client: f.client}
	f.client.conns.Store(f.conn, cc)

	buf, err := packet.PackMessage(&packet.Message{Route: 1, Seq: 7, Buffer: []byte(`{"n":42}`)})
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}
	f.client.handleReceive(f.conn, buf)

	select {
	case n := <-result:
		if n != 42 {
			t.Fatalf("unexpected parsed value: %d", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("route handler not invoked")
	}
}

func TestClientHandleReceiveDefaultRoute(t *testing.T) {
	f := newClientFixture(t)

	result := make(chan int32, 1)
	f.client.Proxy().SetDefaultRouteHandler(func(ctx *Context) { result <- ctx.Route() })
	f.start(t)

	cc := &Conn{conn: f.conn, client: f.client}
	f.client.conns.Store(f.conn, cc)

	buf, err := packet.PackMessage(&packet.Message{Route: 99, Seq: 1, Buffer: []byte(`{}`)})
	if err != nil {
		t.Fatalf("pack message failed: %v", err)
	}
	f.client.handleReceive(f.conn, buf)

	select {
	case route := <-result:
		if route != 99 {
			t.Fatalf("unexpected route: %d", route)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("default route handler not invoked")
	}
}

func TestClientHandleReceiveEdgeCases(t *testing.T) {
	f := newClientFixture(t)
	f.start(t)

	// An unregistered connection is ignored and its buffer released.
	f.client.handleReceive(newTConn(404), buffer.NewBytes([]byte{1, 2, 3}, true))

	// A registered connection with a malformed buffer is handled gracefully.
	cc := &Conn{conn: f.conn, client: f.client}
	f.client.conns.Store(f.conn, cc)
	f.client.handleReceive(f.conn, buffer.NewBytes([]byte{1, 2, 3}, true))
}

func TestClientHandleDisconnect(t *testing.T) {
	f := newClientFixture(t)

	ch := make(chan int64, 1)
	f.client.Proxy().AddEventListener(cluster.Disconnect, func(conn *Conn) { ch <- conn.ID() })
	f.start(t)

	cc := &Conn{conn: f.conn, client: f.client}
	f.client.conns.Store(f.conn, cc)

	f.client.handleDisconnect(f.conn)

	select {
	case id := <-ch:
		if id != f.conn.ID() {
			t.Fatalf("unexpected conn: %d", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("disconnect event not received")
	}

	if _, ok := f.client.conns.Load(f.conn); ok {
		t.Fatal("conn must be removed from the table")
	}

	// A disconnect for an unregistered connection is ignored.
	f.client.handleDisconnect(newTConn(404))
}

func TestClientRegistrationWhileWorking(t *testing.T) {
	f := newClientFixture(t)
	f.start(t)

	// All registration entry points are ignored while working.
	f.client.Proxy().AddRouteHandler(1, func(ctx *Context) {})
	f.client.Proxy().AddEventListener(cluster.Connect, func(conn *Conn) {})
	f.client.Proxy().SetDefaultRouteHandler(func(ctx *Context) {})

	if handlers := f.client.routes.Load().(map[int32][]RouteHandler); len(handlers) != 0 {
		t.Fatalf("route handlers must not be registered while working: %v", handlers)
	}
	if handlers := f.client.events.Load().(map[cluster.Event][]EventHandler); len(handlers) != 0 {
		t.Fatalf("event handlers must not be registered while working: %v", handlers)
	}
	if handler := f.client.defaultRouteHandler.Load().(RouteHandler); handler != nil {
		t.Fatal("default route handler must not be set while working")
	}

	// Only the destroy hook may be added while working.
	f.client.Proxy().AddHookListener(cluster.Destroy, func(p *Proxy) {})
	if handlers := f.client.hooks.Load().(map[cluster.Hook][]HookHandler); len(handlers[cluster.Destroy]) != 1 {
		t.Fatal("destroy hook must be registered while working")
	}
}

func TestClientSetDefaultRouteHandlerTwice(t *testing.T) {
	c := NewClient(WithClient(&tNetwork{conn: newTConn(1)}), WithCodec(json.DefaultCodec))

	c.setDefaultRouteHandler(func(ctx *Context) {})
	first := c.defaultRouteHandler.Load().(RouteHandler)

	// The second registration must be ignored.
	c.setDefaultRouteHandler(func(ctx *Context) {})
	second := c.defaultRouteHandler.Load().(RouteHandler)

	if first == nil || second == nil {
		t.Fatal("default route handler must be set")
	}
}

func TestClientProxyGetters(t *testing.T) {
	f := newClientFixture(t)

	if f.client.Proxy().ID() != "test-client-id" {
		t.Fatalf("unexpected id: %s", f.client.Proxy().ID())
	}
	if f.client.Proxy().Name() != "test-client-name" {
		t.Fatalf("unexpected name: %s", f.client.Proxy().Name())
	}
	if f.client.Proxy().Client() != f.network {
		t.Fatal("unexpected network client")
	}
}

func TestClientConn(t *testing.T) {
	f := newClientFixture(t)
	cc := &Conn{conn: f.conn, client: f.client}

	if cc.ID() != 1 {
		t.Fatalf("unexpected id: %d", cc.ID())
	}

	cc.Bind(100)
	if cc.UID() != 100 {
		t.Fatalf("unexpected uid: %d", cc.UID())
	}
	cc.Unbind()
	if cc.UID() != 0 {
		t.Fatalf("uid must be cleared, got %d", cc.UID())
	}

	cc.SetAttr("k", "v")
	if got := cc.GetAttr("k").String(); got != "v" {
		t.Fatalf("unexpected attr: %s", got)
	}
	cc.DelAttr("k")
	if got := cc.GetAttr("k").String(); got != "" {
		t.Fatalf("attr must be deleted, got %s", got)
	}

	if ip, err := cc.LocalIP(); err != nil || ip != "127.0.0.1" {
		t.Fatalf("unexpected local ip: %s, err: %v", ip, err)
	}
	if _, err := cc.LocalAddr(); err != nil {
		t.Fatalf("local addr failed: %v", err)
	}
	if ip, err := cc.RemoteIP(); err != nil || ip != "10.0.0.1" {
		t.Fatalf("unexpected remote ip: %s, err: %v", ip, err)
	}
	if _, err := cc.RemoteAddr(); err != nil {
		t.Fatalf("remote addr failed: %v", err)
	}

	if err := cc.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if !f.conn.closed.Load() {
		t.Fatal("conn must be closed")
	}
}

func TestClientConnPush(t *testing.T) {
	f := newClientFixture(t)
	cc := &Conn{conn: f.conn, client: f.client}

	// A raw byte payload is passed through unchanged.
	if err := cc.Push(&cluster.Message{Seq: 1, Route: 1, Data: []byte("raw")}); err != nil {
		t.Fatalf("push failed: %v", err)
	}
	<-f.conn.pushed

	// A structured payload is encoded by the codec.
	if err := cc.Push(&cluster.Message{Seq: 2, Route: 2, Data: map[string]string{"k": "v"}}); err != nil {
		t.Fatalf("push failed: %v", err)
	}
	<-f.conn.pushed

	// A nil payload is allowed.
	if err := cc.Push(&cluster.Message{Seq: 3, Route: 3}); err != nil {
		t.Fatalf("push failed: %v", err)
	}
	<-f.conn.pushed
}

func TestClientConnPushErrors(t *testing.T) {
	f := newClientFixture(t)
	cc := &Conn{conn: f.conn, client: f.client}

	// A codec failure is propagated.
	f.client.opts.codec = tCodec{}
	if err := cc.Push(&cluster.Message{Data: map[string]string{"k": "v"}}); err == nil {
		t.Fatal("expect a codec error")
	}
	f.client.opts.codec = json.DefaultCodec

	// An encryptor failure is propagated.
	f.client.opts.encryptor = &tEncryptor{encryptErr: errors.New("encrypt failed")}
	if err := cc.Push(&cluster.Message{Data: []byte("raw")}); err == nil {
		t.Fatal("expect an encryptor error")
	}
	f.client.opts.encryptor = nil

	// A connection push failure is propagated.
	f.conn.pushErr = errors.New("push failed")
	if err := cc.Push(&cluster.Message{Data: []byte("raw")}); err == nil {
		t.Fatal("expect a push error")
	}
}

func TestClientContext(t *testing.T) {
	f := newClientFixture(t)
	f.conn.uid = 100
	cc := &Conn{conn: f.conn, client: f.client}

	buf := buffer.NewBytes([]byte(`{"n":7}`), true)
	ctx := &Context{ctx: context.Background(), conn: cc, route: 5, seq: 6, buf: buf}

	if ctx.Context() == nil {
		t.Fatal("context must not be nil")
	}
	if ctx.CID() != 1 || ctx.UID() != 100 {
		t.Fatalf("unexpected ids: cid=%d uid=%d", ctx.CID(), ctx.UID())
	}
	if ctx.Conn() != cc {
		t.Fatal("unexpected conn")
	}
	if ctx.Route() != 5 || ctx.Seq() != 6 {
		t.Fatalf("unexpected route/seq: %d/%d", ctx.Route(), ctx.Seq())
	}
	if ctx.Data() == nil {
		t.Fatal("data must not be nil")
	}

	var out struct {
		N int `json:"n"`
	}
	if err := ctx.Parse(&out); err != nil || out.N != 7 {
		t.Fatalf("parse failed: %v, value: %d", err, out.N)
	}

	// Parse goes through the encryptor when one is configured.
	f.client.opts.encryptor = &tEncryptor{}
	if err := ctx.Parse(&out); err != nil || out.N != 7 {
		t.Fatalf("parse with encryptor failed: %v, value: %d", err, out.N)
	}

	// A decryption failure is propagated.
	f.client.opts.encryptor = &tEncryptor{decryptErr: errors.New("decrypt failed")}
	if err := ctx.Parse(&out); err == nil {
		t.Fatal("expect a decrypt error")
	}
	f.client.opts.encryptor = nil

	// An unmarshal failure is propagated.
	f.client.opts.codec = tCodec{}
	if err := ctx.Parse(&out); err == nil {
		t.Fatal("expect an unmarshal error")
	}
}

func TestClientAddHookListenerBeforeStart(t *testing.T) {
	f := newClientFixture(t)

	var count atomic.Int32
	f.client.addHookListener(cluster.Init, func(p *Proxy) { count.Add(1) })
	f.client.runHookFunc(cluster.Init)

	if count.Load() != 1 {
		t.Fatalf("hook must run once, got %d", count.Load())
	}

	// Running a hook without any listener is a no-op.
	f.client.runHookFunc(cluster.Destroy)
}

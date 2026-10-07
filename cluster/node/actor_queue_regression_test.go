package node

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/queue"
	"github.com/dobyte/due/v2/errors"
)

type actorQueueBaseContext = Context

// actorQueueCompletionContext reports completion after the underlying context has been recycled.
type actorQueueCompletionContext struct {
	actorQueueBaseContext
	done chan<- struct{}
}

func (c *actorQueueCompletionContext) compareVersionRecycle(version int32) {
	c.actorQueueBaseContext.compareVersionRecycle(version)
	c.done <- struct{}{}
}

func (c *actorQueueCompletionContext) release() {
	c.actorQueueBaseContext.release()
	c.done <- struct{}{}
}

// newActorQueueFixture starts an actor without external services or background watchers.
func newActorQueueFixture(tb testing.TB, size int32, timeout time.Duration) (*Actor, *Node) {
	tb.Helper()

	n := &Node{router: &Router{}}
	n.reqPool = &sync.Pool{New: func() any { return &request{node: n} }}
	n.evtPool = &sync.Pool{New: func() any { return &event{node: n} }}
	n.scheduler = newScheduler(n)

	act := &Actor{
		opts:         &actorOptions{kind: "queue", id: "1"},
		pid:          "queue/1",
		scheduler:    n.scheduler,
		rw:           &sync.RWMutex{},
		taskQueue:    queue.NewTasker(1, timeout),
		messageQueue: queue.NewQueue[Context](size, timeout),
		routes:       map[int32]RouteHandler{1: func(Context) {}},
	}
	act.events.Store(cluster.Connect, EventHandler(func(Context) {}))
	act.state.Store(started)
	n.scheduler.actors.Store(act.PID(), act)

	stopped := make(chan struct{})
	go func() {
		act.dispatch()
		close(stopped)
	}()

	tb.Cleanup(func() {
		act.Destroy()

		select {
		case <-stopped:
		case <-time.After(2 * time.Second):
			tb.Error("the actor dispatcher did not stop")
		}
	})

	return act, n
}

// actorQueueContext obtains a real request or event from the node's pool.
func actorQueueContext(n *Node, kind Kind) Context {
	if kind == Event {
		ctx := n.evtPool.Get().(*event)
		ctx.event = cluster.Connect
		return ctx
	}

	ctx := n.reqPool.Get().(*request)
	ctx.route = 1
	return ctx
}

// TestActorQueueSequentialWritesDoNotAllocateTimers verifies that an empty actor queue does not
// enter the timeout path because of messages it has already consumed.
func TestActorQueueSequentialWritesDoNotAllocateTimers(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
	}{
		{name: "request", kind: Request},
		{name: "event", kind: Event},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const capacity = 2048
			act, n := newActorQueueFixture(t, capacity, time.Second)

			done := make(chan struct{}, 1)
			ctx := &actorQueueCompletionContext{done: done}
			deadline := time.NewTimer(10 * time.Second)
			t.Cleanup(func() { deadline.Stop() })
			write := func() {
				ctx.actorQueueBaseContext = actorQueueContext(n, tt.kind)
				if err := act.Next(ctx); err != nil {
					ctx.actorQueueBaseContext.release()
					t.Fatalf("write failed: %v", err)
				}

				select {
				case <-done:
				case <-deadline.C:
					t.Fatal("the actor did not recycle the context")
				}
			}

			for i := 0; i < 2*capacity; i++ {
				write()
			}
			allocs := testing.AllocsPerRun(256, write)
			// Pool replenishment may allocate occasionally; a timer on every write allocates
			// multiple objects per operation and must not be hidden by that allowance.
			if allocs >= 1 {
				t.Fatalf("writes to a drained queue allocate %.2f objects per operation", allocs)
			}
		})
	}
}

// TestActorQueueFullWriteTimeout verifies that releasing consumed capacity preserves timeouts
// for a genuinely full queue and that a failed write does not prevent later delivery.
func TestActorQueueFullWriteTimeout(t *testing.T) {
	const capacity = 2
	act, n := newActorQueueFixture(t, capacity, 20*time.Millisecond)
	entered := make(chan struct{})
	resume := make(chan struct{})
	var resumeOnce sync.Once
	t.Cleanup(func() { resumeOnce.Do(func() { close(resume) }) })
	var calls atomic.Int32
	act.routes[1] = func(Context) {
		if calls.Add(1) == 1 {
			close(entered)
			<-resume
		}
	}

	done := make(chan struct{}, capacity+2)
	write := func(kind Kind) error {
		ctx := &actorQueueCompletionContext{actorQueueBaseContext: actorQueueContext(n, kind), done: done}
		if err := act.Next(ctx); err != nil {
			ctx.actorQueueBaseContext.release()
			return err
		}
		return nil
	}

	if err := write(Request); err != nil {
		t.Fatalf("initial write failed: %v", err)
	}
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("the first handler did not start")
	}
	for i := 0; i < capacity; i++ {
		if err := write(Event); err != nil {
			t.Fatalf("filling the queue failed: %v", err)
		}
	}
	if err := write(Request); !errors.Is(err, errors.ErrWriteTimeout) {
		t.Fatalf("a full queue must report ErrWriteTimeout, got %v", err)
	}

	resumeOnce.Do(func() { close(resume) })
	for i := 0; i < capacity+1; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("queued messages did not complete after resuming the actor")
		}
	}
	if err := write(Request); err != nil {
		t.Fatalf("delivery after a write timeout failed: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the actor did not process the final request")
	}
	if calls.Load() != 2 {
		t.Fatalf("expected two successful requests, got %d", calls.Load())
	}
}

// BenchmarkActorQueueRoundTrip measures steady actor queue costs after historical traffic has
// exceeded its capacity, using real pooled request and event contexts.
func BenchmarkActorQueueRoundTrip(b *testing.B) {
	for _, kind := range []Kind{Request, Event} {
		name := "request"
		if kind == Event {
			name = "event"
		}
		b.Run(name, func(b *testing.B) {
			const capacity = 64
			act, n := newActorQueueFixture(b, capacity, time.Second)
			done := make(chan struct{}, 1)
			ctx := &actorQueueCompletionContext{done: done}
			write := func() {
				ctx.actorQueueBaseContext = actorQueueContext(n, kind)
				if err := act.Next(ctx); err != nil {
					ctx.actorQueueBaseContext.release()
					b.Fatalf("write failed: %v", err)
				}
				<-done
			}
			for i := 0; i < 2*capacity; i++ {
				write()
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				write()
			}
		})
	}
}

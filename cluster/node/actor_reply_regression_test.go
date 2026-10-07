package node

import (
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
)

const (
	actorReplyRoute        int32 = 1
	actorReplyBarrierRoute int32 = 2
	actorReplyUID          int64 = 100
	actorReplySeq          int32 = 42
)

// actorReplyObservation captures a context before it is returned to the request pool.
type actorReplyObservation struct {
	pid      string
	nid      string
	uid      int64
	seq      int32
	route    int32
	payload  string
	parseErr error
	replyErr error
}

// actorReplyCounts records handler calls on a single actor's dispatcher.
type actorReplyCounts struct {
	requests   int
	replies    int
	unexpected int
}

// waitActorReplyRegression waits for a synchronization signal with a bounded timeout.
func waitActorReplyRegression[T any](t *testing.T, ch <-chan T) T {
	t.Helper()

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case value := <-ch:
		return value
	case <-timer.C:
		t.Fatal("timed out waiting for the actor reply regression signal")
		var zero T
		return zero
	}
}

// newActorReplyRegressionActor registers handlers before any message is delivered.
func newActorReplyRegressionActor(t *testing.T, n *Node, id string, routes map[int32]RouteHandler) *Actor {
	t.Helper()

	actor, err := n.Proxy().Spawn(newStubProcessorCreator(), WithActorKind("reply"), WithActorID(id))
	if err != nil {
		t.Fatalf("spawn failed: %v", err)
	}
	t.Cleanup(func() { actor.Destroy() })

	for route, handler := range routes {
		actor.AddRouteHandler(route, handler)
	}

	registered := make(chan struct{})
	if err := actor.Invoke(func() { close(registered) }); err != nil {
		t.Fatalf("handler registration barrier failed: %v", err)
	}
	waitActorReplyRegression(t, registered)

	return actor
}

// observeActorReply captures the original source and the message metadata.
func observeActorReply(ctx Context) actorReplyObservation {
	observation := actorReplyObservation{
		pid:   ctx.(*request).pid,
		nid:   ctx.NID(),
		uid:   ctx.UID(),
		seq:   ctx.Seq(),
		route: ctx.Route(),
	}
	observation.parseErr = ctx.Parse(&observation.payload)

	return observation
}

// checkActorReplyObservation verifies the source, payload and unchanged reply metadata.
func checkActorReplyObservation(t *testing.T, n *Node, observation actorReplyObservation, pid, payload string) {
	t.Helper()

	if observation.parseErr != nil || observation.replyErr != nil {
		t.Fatalf("parse failed: %v; response failed: %v", observation.parseErr, observation.replyErr)
	}
	if observation.pid != pid || observation.nid != n.opts.id {
		t.Errorf("unexpected message source: actor %q, node %q; want actor %q, node %q", observation.pid, observation.nid, pid, n.opts.id)
	}
	if observation.uid != actorReplyUID || observation.seq != actorReplySeq || observation.route != actorReplyRoute {
		t.Errorf("unexpected message metadata: uid %d, seq %d, route %d", observation.uid, observation.seq, observation.route)
	}
	if observation.payload != payload {
		t.Errorf("unexpected payload: %q; want %q", observation.payload, payload)
	}
}

// TestActorDeliverResponseDoesNotFeedback verifies direct and deferred responses do not re-enter
// the route that received a direct actor delivery.
func TestActorDeliverResponseDoesNotFeedback(t *testing.T) {
	for _, tt := range []struct {
		name     string
		deferred bool
	}{
		{name: "response"},
		{name: "deferred response", deferred: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n := newTestNode(t)
			setWorking(n)

			responses := make(chan actorReplyObservation, 1)
			checkpoints := make(chan int, 1)
			calls := 0
			actor := newActorReplyRegressionActor(t, n, "direct", map[int32]RouteHandler{
				actorReplyRoute: func(ctx Context) {
					calls++
					// Reply only to the first message so a regression cannot loop indefinitely.
					if calls != 1 {
						return
					}

					observation := observeActorReply(ctx)
					respond := func() {
						observation.replyErr = ctx.Response("reply")
						responses <- observation
					}
					if tt.deferred {
						ctx.Defer(respond)
					} else {
						respond()
					}
				},
				actorReplyBarrierRoute: func(ctx Context) { checkpoints <- calls },
			})

			if err := actor.Deliver(actorReplyUID, &cluster.Message{Route: actorReplyRoute, Seq: actorReplySeq, Data: "request"}); err != nil {
				t.Fatalf("deliver failed: %v", err)
			}
			checkActorReplyObservation(t, n, waitActorReplyRegression(t, responses), "", "request")

			// The same-queue barrier follows any message enqueued by Response before it returned.
			if err := actor.Deliver(actorReplyUID, &cluster.Message{Route: actorReplyBarrierRoute}); err != nil {
				t.Fatalf("message barrier failed: %v", err)
			}
			if got := waitActorReplyRegression(t, checkpoints); got != 1 {
				t.Fatalf("a single delivery invoked the handler %d times; want 1", got)
			}
		})
	}
}

// TestActorPushReplyDoesNotFeedback verifies a reply reaches the sending actor once, including
// when it is also the receiving actor, and that replying to the response does not feed back.
func TestActorPushReplyDoesNotFeedback(t *testing.T) {
	for _, tt := range []struct {
		name string
		self bool
	}{
		{name: "another actor"},
		{name: "self", self: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n := newTestNode(t)
			var receiver *Actor
			var forwardErr error
			n.router.AddRouteHandler(actorReplyRoute, func(ctx Context) {
				forwardErr = receiver.Next(ctx)
			})
			setWorking(n)

			requests := make(chan actorReplyObservation, 1)
			replies := make(chan actorReplyObservation, 1)
			checkpoints := make(chan actorReplyCounts, 1)
			newActor := func(id string, counts *actorReplyCounts) *Actor {
				return newActorReplyRegressionActor(t, n, id, map[int32]RouteHandler{
					actorReplyRoute: func(ctx Context) {
						observation := observeActorReply(ctx)
						switch observation.payload {
						case "request":
							counts.requests++
							if counts.requests == 1 {
								observation.replyErr = ctx.Response("reply")
								requests <- observation
							}
						case "reply":
							counts.replies++
							if counts.replies == 1 {
								observation.replyErr = ctx.Response("terminal")
								replies <- observation
							}
						default:
							// A broken response origin can enqueue one terminal message, then stops.
							counts.unexpected++
						}
					},
					actorReplyBarrierRoute: func(ctx Context) { checkpoints <- *counts },
				})
			}

			source := newActor("source", &actorReplyCounts{})
			receiver = source
			if !tt.self {
				receiver = newActor("receiver", &actorReplyCounts{})
			}

			if err := source.Push(actorReplyUID, &cluster.Message{Route: actorReplyRoute, Seq: actorReplySeq, Data: "request"}); err != nil {
				t.Fatalf("push failed: %v", err)
			}
			pushed := waitActorReplyRegression(t, n.router.receive())
			checkActorReplyObservation(t, n, observeActorReply(pushed), source.PID(), "request")
			n.router.handle(pushed)
			if forwardErr != nil {
				t.Fatalf("forward to actor failed: %v", forwardErr)
			}

			checkActorReplyObservation(t, n, waitActorReplyRegression(t, requests), source.PID(), "request")
			checkActorReplyObservation(t, n, waitActorReplyRegression(t, replies), "", "reply")

			if err := source.Deliver(actorReplyUID, &cluster.Message{Route: actorReplyBarrierRoute}); err != nil {
				t.Fatalf("source message barrier failed: %v", err)
			}
			wantSource := actorReplyCounts{replies: 1}
			if tt.self {
				wantSource.requests = 1
			}
			if got := waitActorReplyRegression(t, checkpoints); got != wantSource {
				t.Errorf("unexpected source handler calls: %+v; want %+v", got, wantSource)
			}

			if !tt.self {
				if err := receiver.Deliver(actorReplyUID, &cluster.Message{Route: actorReplyBarrierRoute}); err != nil {
					t.Fatalf("receiver message barrier failed: %v", err)
				}
				if got := waitActorReplyRegression(t, checkpoints); got != (actorReplyCounts{requests: 1}) {
					t.Errorf("unexpected receiver handler calls: %+v", got)
				}
			}
		})
	}
}

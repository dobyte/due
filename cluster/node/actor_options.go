package node

import "time"

// actorOptions holds the options of an [Actor].
type actorOptions struct {
	id                  string        // Actor ID
	kind                string        // Actor kind
	args                []any         // Arguments passed to the Processor
	wait                bool          // Whether the actor must be waited for
	dispatch            bool          // Whether the actor accepts scheduling from the scheduler
	taskQueueSize       int32         // Task queue capacity
	taskWriteTimeout    time.Duration // Timeout for writing a task to the task queue
	messageQueueSize    int32         // Message queue capacity
	messageWriteTimeout time.Duration // Timeout for writing a message to the message queue
}

// ActorOption is a configuration function of an [Actor].
type ActorOption func(o *actorOptions)

// defaultActorOptions creates the default actor options.
func defaultActorOptions() *actorOptions {
	return &actorOptions{
		wait:                true,
		dispatch:            true,
		taskQueueSize:       1024,
		taskWriteTimeout:    3 * time.Second,
		messageQueueSize:    1024,
		messageWriteTimeout: 3 * time.Second,
	}
}

// WithActorID sets the actor ID.
func WithActorID(id string) ActorOption {
	return func(o *actorOptions) { o.id = id }
}

// WithActorKind sets the actor kind.
func WithActorKind(kind string) ActorOption {
	return func(o *actorOptions) { o.kind = kind }
}

// WithActorArgs sets the arguments passed to the Processor.
func WithActorArgs(args ...any) ActorOption {
	return func(o *actorOptions) { o.args = append(o.args, args...) }
}

// WithActorNonWait marks the actor as not needing to be waited for, so that the Node component does
// not wait for this actor to finish when shutting down.
func WithActorNonWait() ActorOption {
	return func(o *actorOptions) { o.wait = false }
}

// WithActorNonDispatch makes the actor non-schedulable.
func WithActorNonDispatch() ActorOption {
	return func(o *actorOptions) { o.dispatch = false }
}

// WithActorTaskQueueSize sets the task queue capacity.
func WithActorTaskQueueSize(size int32) ActorOption {
	return func(o *actorOptions) { o.taskQueueSize = size }
}

// WithActorTaskWriteTimeout sets the timeout for writing a task to the task queue.
func WithActorTaskWriteTimeout(timeout time.Duration) ActorOption {
	return func(o *actorOptions) { o.taskWriteTimeout = timeout }
}

// WithActorMessageQueueSize sets the message queue capacity.
func WithActorMessageQueueSize(size int32) ActorOption {
	return func(o *actorOptions) { o.messageQueueSize = size }
}

// WithActorMessageWriteTimeout sets the timeout for writing a message to the message queue.
func WithActorMessageWriteTimeout(timeout time.Duration) ActorOption {
	return func(o *actorOptions) { o.messageWriteTimeout = timeout }
}

package job

import (
	"context"
	"time"
)

// Handler executes one run locally.
type Handler interface {
	Execute(ctx context.Context, request *Request) (*Result, error)
}

// Request is the immutable input passed to a Handler.
type Request struct {
	RunID        string
	JobID        string
	TaskType     string
	Payload      []byte
	Attempt      int
	FencingToken int64
	Deadline     time.Time
}

// Result is the result persisted after a successful Handler call.
type Result struct {
	Output    []byte
	ResultRef string
}

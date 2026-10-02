package errors

import (
	"fmt"
	"io"

	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/core/stack"
)

var (
	ErrNil                     = New("nil")
	ErrInvalidGID              = New("invalid gate id")
	ErrInvalidNID              = New("invalid node id")
	ErrInvalidMessage          = New("invalid message")
	ErrInvalidReader           = New("invalid reader")
	ErrNotFoundSession         = New("not found session")
	ErrInvalidSessionKind      = New("invalid session kind")
	ErrReceiveTargetEmpty      = New("the receive target is empty")
	ErrInvalidArgument         = New("invalid argument")
	ErrNotFoundRoute           = New("not found route")
	ErrNotFoundEvent           = New("not found event")
	ErrNotFoundEndpoint        = New("not found endpoint")
	ErrNotFoundUserLocation    = New("not found user's location")
	ErrClientShut              = New("client is shut")
	ErrConnectionOpened        = New("connection is opened")
	ErrConnectionHanged        = New("connection is hanged")
	ErrConnectionClosed        = New("connection is closed")
	ErrConnectionAlived        = New("connection is alived")
	ErrConnectionNotAlived     = New("connection is not alived")
	ErrConnectionNotOpened     = New("connection is not opened")
	ErrConnectionNotHanged     = New("connection is not hanged")
	ErrTooManyConnection       = New("too many connection")
	ErrSeqOverflow             = New("seq overflow")
	ErrRouteOverflow           = New("route overflow")
	ErrMessageTooLarge         = New("message too large")
	ErrInvalidDecoder          = New("invalid decoder")
	ErrInvalidScanner          = New("invalid scanner")
	ErrNoOperationPermission   = New("no operation permission")
	ErrInvalidConfigContent    = New("invalid config content")
	ErrNotFoundConfigSource    = New("not found config source")
	ErrConfigStoreFailed       = New("config store failed")
	ErrInvalidFormat           = New("invalid format")
	ErrIllegalRequest          = New("illegal request")
	ErrIllegalOperation        = New("illegal operation")
	ErrInvalidPointer          = New("invalid pointer")
	ErrNotFoundLocator         = New("not found locator")
	ErrUnexpectedEOF           = New("unexpected EOF")
	ErrMissingTransporter      = New("missing transporter")
	ErrMissingDiscovery        = New("missing discovery")
	ErrNotFoundServiceAddress  = New("not found service address")
	ErrUnknownError            = New("unknown error")
	ErrClientClosed            = New("client is closed")
	ErrClientStarted           = New("client is started")
	ErrServerClosed            = New("server is closed")
	ErrServerStarted           = New("server is started")
	ErrActorExists             = New("actor exists")
	ErrActorCreateFailed       = New("actor create failed")
	ErrMissingDispatchStrategy = New("missing dispatch strategy")
	ErrUnregisterRoute         = New("unregistered route")
	ErrNotBindActor            = New("not bind actor")
	ErrNotFoundActor           = New("not found actor")
	ErrSyncerClosed            = New("syncer is closed")
	ErrDeadlineExceeded        = New("deadline exceeded")
	ErrMissingResolver         = New("missing resolver")
	ErrServiceRegisterFailed   = New("service register failed")
	ErrServiceDeregisterFailed = New("service deregister failed")
	ErrRegistryClosed          = New("registry is closed")
	ErrInvalidPublicKey        = New("invalid public key")
	ErrInvalidPrivateKey       = New("invalid private key")
	ErrInvalidSignature        = New("invalid signature")
	ErrNotFoundIPAddress       = New("not found ip address")
	ErrInvalidServiceDesc      = New("invalid service desc")
	ErrInvalidCertFile         = New("invalid cert file")
	ErrMissingCacheInstance    = New("missing cache instance")
	ErrMissingEventbusInstance = New("missing eventbus instance")
	ErrActorNotStarted         = New("actor not started")
	ErrWatcherStopped          = New("watcher stopped")
	ErrWriteTimeout            = New("write timeout")
	ErrQueueHanged             = New("queue is hanged")
	ErrQueueClosed             = New("queue is closed")
	ErrGateShutdown            = New("gate is shutdown")
	ErrNodeShutdown            = New("node is shutdown")
	ErrMeshShutdown            = New("mesh is shutdown")
	ErrCacheClosed             = New("cache is closed")
	ErrIllegalInvoke           = New("illegal invoke")
)

// NewError creates a new error.
//
// The following arguments are recognized: a string sets the text, a [*codes.Code] sets the code,
// and an error sets the underlying error.
func NewError(args ...any) *Error {
	e := &Error{}

	for _, arg := range args {
		switch v := arg.(type) {
		case error:
			e.err = v
		case string:
			e.text = v
		case *codes.Code:
			e.code = v
		}
	}

	return e
}

// NewErrorWithStack creates a new error with a captured stack.
//
// The following arguments are recognized: a string sets the text, a [*codes.Code] sets the code,
// and an error sets the underlying error.
func NewErrorWithStack(args ...any) *Error {
	e := &Error{stack: stack.Callers(1, stack.Full)}

	for _, arg := range args {
		switch v := arg.(type) {
		case error:
			e.err = v
		case string:
			e.text = v
		case *codes.Code:
			e.code = v
		}
	}

	return e
}

// Code returns the error code carried by err, or nil when it carries none.
func Code(err error) *codes.Code {
	if err != nil {
		if e, ok := err.(interface{ Code() *codes.Code }); ok {
			return e.Code()
		}
	}

	return nil
}

// Next returns the next error in the chain.
func Next(err error) error {
	if err == nil {
		return nil
	}

	if e, ok := err.(interface{ Next() error }); ok {
		return e.Next()
	}

	return nil
}

// Cause returns the root cause of err.
func Cause(err error) error {
	if err == nil {
		return nil
	}

	if e, ok := err.(interface{ Cause() error }); ok {
		return e.Cause()
	}

	return err
}

// Stack returns the stack captured by err.
func Stack(err error) *stack.Stack {
	if err == nil {
		return nil
	}

	if e, ok := err.(interface{ Stack() *stack.Stack }); ok {
		return e.Stack()
	}

	return nil
}

// Replace replaces the text of err when it supports replacement and its code matches condition.
func Replace(err error, text string, condition ...codes.Code) error {
	if err == nil {
		return nil
	}

	if e, ok := err.(interface {
		Replace(text string, condition ...codes.Code) error
	}); ok {
		return e.Replace(text, condition...)
	}

	return err
}

type Error struct {
	err   error
	text  string
	code  *codes.Code
	stack *stack.Stack
}

func (e *Error) Error() (text string) {
	if e == nil {
		return
	}

	if e.code != nil && e.code != codes.OK {
		text = e.code.String()
	}

	if e.text != "" {
		if text != "" {
			text += ": "
		}
		text += e.text
	}

	if e.err != nil && e.err.Error() != "" {
		if text != "" {
			text += ": "
		}
		text += e.err.Error()
	}

	return
}

// Code returns the error code.
func (e *Error) Code() *codes.Code {
	if e == nil {
		return nil
	}

	return e.code
}

// Next returns the next error.
func (e *Error) Next() error {
	if e == nil {
		return nil
	}

	return e.err
}

// Cause returns the root cause error.
func (e *Error) Cause() error {
	if e == nil {
		return nil
	}

	if e.err == nil {
		return e
	}

	cause := e.err
	for cause != nil {
		if ce, ok := cause.(interface{ Cause() error }); ok {
			cause = ce.Cause()
		} else {
			break
		}
	}

	return cause
}

// Stack returns the stack.
func (e *Error) Stack() *stack.Stack {
	if e == nil {
		return nil
	}

	return e.stack
}

// Unwrap returns the underlying error.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.err
}

// Replace replaces the text when condition is empty or matches the error code.
func (e *Error) Replace(text string, condition ...*codes.Code) error {
	if e == nil {
		return nil
	}

	if len(condition) == 0 || condition[0] == e.code {
		e.text = text
	}

	return e
}

// String returns the formatted error information.
func (e *Error) String() string {
	return fmt.Sprintf("%+v", e)
}

// error returns the error text used for formatting.
func (e *Error) error() (text string) {
	if e == nil {
		return
	}

	text = e.text
	if text == "" && e.code != codes.OK {
		text = e.code.String()
	}

	return
}

// Format implements [fmt.Formatter].
//
// The %s verb prints the error at the current level, %v prints all error information, and %+v
// prints all error information together with the stack.
func (e *Error) Format(s fmt.State, verb rune) {
	if e == nil {
		return
	}

	switch verb {
	case 'v':
		if s.Flag('+') {
			var (
				i    int
				next error = e
			)

			io.WriteString(s, e.Error()+"\nStack:\n")
			for next != nil {
				i++
				if n, ok := next.(*Error); ok {
					fmt.Fprintf(s, "%d. %s\n", i, n.error())
					for i, f := range n.stack.Frames() {
						fmt.Fprintf(s, "\t%d). %s\n\t%s:%d\n",
							i+1,
							f.Function,
							f.File,
							f.Line,
						)
					}
					next = n.Next()
				} else {
					fmt.Fprintf(s, "%d. %s\n", i, next.Error())
					break
				}
			}
		} else {
			io.WriteString(s, e.Error())
		}
	case 's':
		if e.text != "" {
			io.WriteString(s, e.text)
		} else {
			e.code.Format(s, verb)
		}
	}
}

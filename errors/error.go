package errors

import (
	"fmt"
	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/core/stack"
	"io"
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
	ErrActorExists             = New("actor exists")
	ErrMissingDispatchStrategy = New("missing dispatch strategy")
	ErrUnregisterRoute         = New("unregistered route")
	ErrNotBindActor            = New("not bind actor")
	ErrNotFoundActor           = New("not found actor")
	ErrWriterClosing           = New("writer is closing")
	ErrDeadlineExceeded        = New("deadline exceeded")
	ErrMissingResolver         = New("missing resolver")
)

// NewError 新建一个错误
// 可传入一下参数：
// text : 文本字符串
// code : 错误码
// error: 原生错误
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

// NewErrorWithStack 新建一个带堆栈的错误
// 可传入一下参数：
// text : 文本字符串
// code : 错误码
// error: 原生错误
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

// Code 返回错误码
func Code(err error) *codes.Code {
	if err != nil {
		if e, ok := err.(interface{ Code() *codes.Code }); ok {
			return e.Code()
		}
	}

	return nil
}

// Next 返回下一个错误
func Next(err error) error {
	if err == nil {
		return nil
	}

	if e, ok := err.(interface{ Next() error }); ok {
		return e.Next()
	}

	return nil
}

// Cause 返回根因错误
func Cause(err error) error {
	if err == nil {
		return nil
	}

	if e, ok := err.(interface{ Cause() error }); ok {
		return e.Cause()
	}

	return err
}

// Stack 返回堆栈
func Stack(err error) *stack.Stack {
	if err == nil {
		return nil
	}

	if e, ok := err.(interface{ Stack() *stack.Stack }); ok {
		return e.Stack()
	}

	return nil
}

// Replace 替换文本
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

// Code 返回错误码
func (e *Error) Code() *codes.Code {
	if e == nil {
		return nil
	}

	return e.code
}

// Next 返回下一个错误
func (e *Error) Next() error {
	if e == nil {
		return nil
	}

	return e.err
}

// Cause 返回根因错误
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

// Stack 返回堆栈
func (e *Error) Stack() *stack.Stack {
	if e == nil {
		return nil
	}

	return e.stack
}

// Unwrap 解包错误
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.err
}

// Replace 替换文本
func (e *Error) Replace(text string, condition ...*codes.Code) error {
	if e == nil {
		return nil
	}

	if len(condition) == 0 || condition[0] == e.code {
		e.text = text
	}

	return e
}

// String 格式化错误信息
func (e *Error) String() string {
	return fmt.Sprintf("%+v", e)
}

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

// Format 格式化输出
// %s : 打印本级错误信息
// %v : 打印所有错误信息
// %+v: 打印所有错误信息和堆栈信息
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

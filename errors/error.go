package errors

import (
	"fmt"
	"github.com/dobyte/due/code"
	"github.com/dobyte/due/internal/stack"
	"io"
)

type Error interface {
	error
	// Is 返回当前错误是否等于目标错误
	Is(target error) bool
	// As 返回当前错误是否是某一类错误
	As(target interface{}) bool
	// Code 返回错误码
	Code() code.Code
	// Next 返回下一个错误
	Next() error
	// Cause 返回根因错误
	Cause() error
	// Stack 返回堆栈
	Stack() *stack.Stack
}

// NewError 新建一个错误
// 可传入一下参数：
// text : 文本字符串
// code : 错误码
// error: 原生错误
func NewError(args ...interface{}) error {
	e := &defaultError{
		code:  code.Nil,
		stack: stack.Callers(1, stack.Full),
	}

	for _, arg := range args {
		switch v := arg.(type) {
		case error:
			e.err = v
		case string:
			e.text = v
		case code.Code:
			e.code = v
		}
	}

	return e
}

// Code 返回错误码
func Code(err error) code.Code {
	if err != nil {
		if e, ok := err.(interface{ Code() code.Code }); ok {
			return e.Code()
		}
	}

	return code.Nil
}

// Next 返回下一个错误
func Next(err error) error {
	if err != nil {
		if e, ok := err.(interface{ Next() error }); ok {
			return e.Next()
		}
	}

	return nil
}

// Cause 返回根因错误
func Cause(err error) error {
	if err != nil {
		if e, ok := err.(interface{ Cause() error }); ok {
			return e.Cause()
		}
	}

	return nil
}

// Stack 返回堆栈
func Stack(err error) *stack.Stack {
	if err != nil {
		if e, ok := err.(interface{ Stack() *stack.Stack }); ok {
			return e.Stack()
		}
	}

	return nil
}

var _ Error = &defaultError{}

type defaultError struct {
	err   error
	code  code.Code
	text  string
	stack *stack.Stack
}

func (e *defaultError) Error() (text string) {
	if e == nil {
		return
	}

	text = e.text

	if text == "" && e.code != code.Nil {
		text = e.code.Message()
	}

	if e.err != nil {
		if text != "" {
			text += ": "
		}
		text += e.err.Error()
	}

	return
}

// Is 返回当前错误是否等于目标错误
func (e *defaultError) Is(target error) bool {
	return Is(e, target)
}

// As 返回当前错误是否是某一类错误
func (e *defaultError) As(target interface{}) bool {
	return As(e, target)
}

// Code 返回错误码
func (e *defaultError) Code() code.Code {
	if e == nil {
		return code.Nil
	}

	return e.code
}

// Next 返回下一个错误
func (e *defaultError) Next() error {
	if e == nil {
		return nil
	}

	return e.err
}

// Cause 返回根因错误
func (e *defaultError) Cause() error {
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
func (e *defaultError) Stack() *stack.Stack {
	return e.stack
}

// Unwrap 解包错误
func (e *defaultError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

// String 格式化错误信息
func (e *defaultError) String() string {
	return fmt.Sprintf("%+v", e)
}

func (e *defaultError) error() (text string) {
	if e == nil {
		return
	}

	text = e.text
	if text == "" && e.code != code.Nil {
		text = e.code.Message()
	}

	return
}

// Format 格式化输出
// %s : 打印本级错误信息
// %v : 打印所有错误信息
// %+v: 打印所有错误信息和堆栈信息
func (e *defaultError) Format(s fmt.State, verb rune) {
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
				if n, ok := next.(*defaultError); ok {
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
			e.code.(interface{ Format(fmt.State, rune) }).Format(s, verb)
		}
	}
}

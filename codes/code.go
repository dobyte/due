package codes

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

var (
	OK               = NewCode(0, "ok")
	Canceled         = NewCode(1, "canceled")
	Unknown          = NewCode(2, "unknown")
	InvalidArgument  = NewCode(3, "invalid argument")
	DeadlineExceeded = NewCode(4, "deadline exceeded")
	NotFound         = NewCode(5, "not found")
	InternalError    = NewCode(6, "internal error")
	Unauthorized     = NewCode(7, "unauthorized")
	IllegalInvoke    = NewCode(8, "illegal invoke")
	IllegalRequest   = NewCode(9, "illegal request")
	TooManyRequests  = NewCode(10, "too many requests")
)

type Code struct {
	code    int
	message string
}

// NewCode 新建一个错误码
func NewCode(code int, message ...string) *Code {
	if len(message) > 0 {
		return &Code{code: code, message: message[0]}
	} else {
		return &Code{code: code}
	}
}

// Code 返回错误码
func (c *Code) Code() int {
	return c.code
}

// WithCode 替换新的错误码
func (c *Code) WithCode(code int) *Code {
	return &Code{
		code:    code,
		message: c.message,
	}
}

// Message 返回错误码消息
func (c *Code) Message() string {
	return c.message
}

// WithMessage 替换新的错误码消息
func (c *Code) WithMessage(message string) *Code {
	return &Code{
		code:    c.code,
		message: message,
	}
}

// String 格式化错误码
func (c *Code) String() string {
	return fmt.Sprintf("code error: code = %d desc = %s", c.code, c.message)
}

// Format 格式化输出
// %s : 打印错误码和错误消息
// %v : 打印错误码、错误消息、错误详情
func (c *Code) Format(s fmt.State, verb rune) {
	switch verb {
	case 's':
		if c.message != "" {
			io.WriteString(s, fmt.Sprintf("%d:%s", c.code, c.message))
		} else {
			io.WriteString(s, fmt.Sprintf("%d", c.code))
		}
	case 'v':
		io.WriteString(s, c.String())
	}
}

// Err 转错误消息
func (c *Code) Err() error {
	if c.code == OK.Code() {
		return nil
	}

	return &Error{code: c}
}

type Error struct {
	code *Code
}

// Error error interface implementation
func (e *Error) Error() string {
	return e.code.String()
}

// Convert 将错误信息转换为错误码
func Convert(err error) *Code {
	if err == nil {
		return OK
	}

	if e, ok := err.(interface{ Code() *Code }); ok {
		return e.Code()
	}

	text := err.Error()
	flag := "code error:"
	index := strings.Index(text, flag)

	if index == -1 {
		return Unknown
	}

	after, found := strings.CutPrefix(text[index+len(flag):], " code = ")
	if !found {
		return Unknown
	}

	elements := strings.SplitN(after, " ", 2)
	if len(elements) != 2 {
		return Unknown
	}

	code, err := strconv.Atoi(elements[0])
	if err != nil {
		return Unknown
	}

	after, found = strings.CutPrefix(elements[1], "desc = ")
	if !found {
		return Unknown
	}

	return NewCode(code, after)
}

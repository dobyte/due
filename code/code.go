package code

import (
	"fmt"
	"io"
)

var (
	Nil              = NewCode(-1, "", nil)
	InternalError    = NewCode(1, "internal error", nil)
	InvalidParameter = NewCode(2, "invalid parameter", nil)
	NotLocatedUser   = NewCode(3, "not located user", nil)
)

type Code interface {
	// Code 返回错误码
	Code() int
	// Message 返回错误码消息
	Message() string
	// Detail 返回错误码详情
	Detail() interface{}
	// String 格式化错误码
	String() string
}

type defaultCode struct {
	code    int
	message string
	detail  interface{}
}

func NewCode(code int, message string, detail interface{}) Code {
	return &defaultCode{
		code:    code,
		message: message,
		detail:  detail,
	}
}

// Code 返回错误码
func (c *defaultCode) Code() int {
	return c.code
}

// Message 返回错误码消息
func (c *defaultCode) Message() string {
	return c.message
}

// Detail 返回错误码详情
func (c *defaultCode) Detail() interface{} {
	return c.detail
}

// String 格式化错误码
func (c *defaultCode) String() string {
	if c.message != "" {
		if c.detail != nil {
			return fmt.Sprintf("%d:%s %v", c.code, c.message, c.detail)
		}

		return fmt.Sprintf("%d:%s", c.code, c.message)
	}

	return fmt.Sprintf("%d", c.code)
}

// Format 格式化输出
// %s : 打印错误码和错误消息
// %v : 打印错误码、错误消息、错误详情
func (c *defaultCode) Format(s fmt.State, verb rune) {
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

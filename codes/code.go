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
	Illegal          = NewCode(8, "illegal")

	NotLocatedUser = NewCode(3, "not located user")
)

type Code struct {
	code    int
	message string
}

func NewCode(code int, message string) *Code {
	return &Code{
		code:    code,
		message: message,
	}
}

// Code 返回错误码
func (c *Code) Code() int {
	return c.code
}

// Message 返回错误码消息
func (c *Code) Message() string {
	return c.message
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

func (c *Code) ErrWith() {

}

type Error struct {
	code *Code
}

func (e *Error) Error() string {
	return e.code.String()
}

func Convert(err error) *Code {
	if err == nil {
		return OK
	}

	text := err.Error()

	after, found := strings.CutPrefix(text, "code error:")
	if !found {
		return Unknown
	}

	after, found = strings.CutPrefix(after, " code = ")
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

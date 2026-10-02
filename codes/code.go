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

// NewCode returns a new Code with the given code and an optional message.
func NewCode(code int, message ...string) *Code {
	if len(message) > 0 {
		return &Code{code: code, message: message[0]}
	} else {
		return &Code{code: code}
	}
}

// Code returns the error code.
func (c *Code) Code() int {
	return c.code
}

// WithCode returns a copy of c with the code replaced.
func (c *Code) WithCode(code int) *Code {
	return &Code{
		code:    code,
		message: c.message,
	}
}

// Message returns the error code message.
func (c *Code) Message() string {
	return c.message
}

// WithMessage returns a copy of c with the message replaced.
func (c *Code) WithMessage(message string) *Code {
	return &Code{
		code:    c.code,
		message: message,
	}
}

// WithMessagef returns a copy of c with the message formatted by format and a.
func (c *Code) WithMessagef(format string, a ...any) *Code {
	return c.WithMessage(fmt.Sprintf(format, a...))
}

// String returns the formatted error code.
func (c *Code) String() string {
	return fmt.Sprintf("code error: code = %d desc = %s", c.code, c.message)
}

// Format implements [fmt.Formatter].
//
// The %s verb prints the error code and the error message, and %v prints the error code, the
// error message and the error detail.
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

// Err converts the code to an error, or returns nil when c is [OK].
func (c *Code) Err() error {
	if c.code == OK.Code() {
		return nil
	}

	return &Error{code: c}
}

type Error struct {
	code *Code
}

// Error implements the error interface.
func (e *Error) Error() string {
	return e.code.String()
}

// Convert converts err to a code. It returns [OK] when err is nil, the carried code when err
// exposes one, and [Unknown] when err cannot be parsed.
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

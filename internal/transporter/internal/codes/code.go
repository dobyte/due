package codes

import (
	"github.com/dobyte/due/v2/errors"
)

const (
	OK              uint16 = iota // 成功
	NotFoundSession               // 未找到会话连接
	InternalError                 // 内部错误
)

// ErrorToCode 错误转错误码
func ErrorToCode(err error) uint16 {
	switch {
	case err == nil:
		return OK
	case errors.Is(err, errors.ErrNotFoundSession):
		return NotFoundSession
	default:
		return InternalError
	}
}

func CodeToError(code uint16) error {
	switch code {
	case OK:
		return nil
	case NotFoundSession:
		return errors.ErrNotFoundSession
	default:
		return errors.ErrUnknownError
	}
}

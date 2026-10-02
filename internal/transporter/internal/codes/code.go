package codes

import (
	"github.com/dobyte/due/v2/errors"
)

const (
	OK              uint16 = iota // Success
	NotFoundSession               // Session connection not found
	InternalError                 // Internal error
)

// ErrorToCode converts an error to its error code.
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

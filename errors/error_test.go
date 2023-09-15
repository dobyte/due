package errors_test

import (
	"fmt"
	"github.com/symsimmy/due/code"
	"github.com/symsimmy/due/errors"
	"testing"
)

func TestNew(t *testing.T) {
	innerErr := errors.NewError(
		"db error",
		code.NewCode(2, "internal error", ""),
		errors.New("std not found"),
	)

	err := errors.NewError(
		//"not found",
		code.NewCode(1, "not found", ""),
		innerErr,
	)

	t.Log(err)
	t.Log(err.(errors.Error).Code())
	t.Log(err.(errors.Error).Next())
	t.Log(err.(errors.Error).Cause())
	fmt.Println(fmt.Sprintf("%+v", err))
}

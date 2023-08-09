package errors_test

import (
	"fmt"
	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/errors"
	"testing"
)

func TestNew(t *testing.T) {
	innerErr := errors.NewError(
		"db error",
		codes.NewCode(2, "core error"),
		errors.New("std not found"),
	)

	err := errors.NewError(
		//"not found",
		codes.NewCode(1, "not found"),
		innerErr,
	)

	t.Log(err)
	t.Log(err.Code())
	t.Log(err.Next())
	t.Log(err.Cause())
	fmt.Println(fmt.Sprintf("%+v", err))
}

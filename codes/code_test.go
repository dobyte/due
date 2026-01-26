package codes_test

import (
	"strings"
	"testing"

	"github.com/dobyte/due/v2/codes"
	"github.com/dobyte/due/v2/errors"
)

func TestConvert(t *testing.T) {
	var (
		err1  = errors.NewError(codes.InternalError, errors.New("file not exists"))
		err2  = errors.New(err1.Error())
		code1 = codes.Convert(err1)
		code2 = codes.Convert(err2)
	)

	t.Log(code1.Code())
	t.Log(code1.Message())
	t.Log(err1.Error())
	t.Log(err2.Error())
	t.Log(code2.Code())
	t.Log(code2.Message())

	if parts := strings.SplitN(code2.Message(), ": ", 2); len(parts) > 0 {
		t.Log(parts)
	}
}

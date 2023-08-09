package codes_test

import (
	"github.com/dobyte/due/v2/codes"
	"testing"
)

func TestConvert(t *testing.T) {
	code := codes.Convert(codes.NotFound.Err())

	t.Log(code)
}

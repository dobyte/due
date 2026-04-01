package xconv_test

import (
	"testing"

	"github.com/dobyte/due/v2/utils/xconv"
)

type Status int32

func TestGenericNumber(t *testing.T) {
	t.Log(xconv.GenericNumbers[Status]([]int{1, 2, 3}))
	t.Log(xconv.GenericNumbers[Status]([]string{"1", "2", "3"}))
}

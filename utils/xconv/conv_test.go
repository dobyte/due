package xconv_test

import (
	"github.com/dobyte/due/utils/xconv"
	"math"
	"math/cmplx"
	"testing"
	"time"
)

func TestInt64(t *testing.T) {
	a := cmplx.Exp(1i*math.Pi) + 20
	t.Log(xconv.Int64(time.Now()))
	t.Log(xconv.Int64(func() {}))
	t.Log(xconv.Int64(&a))
}

func TestString(t *testing.T) {
	t.Log(xconv.String(1))
	t.Log(xconv.String(int8(1)))

	var a = int64(1)
	var b = 1.1
	var c = &b

	t.Log(xconv.String(&a))
	t.Log(*c)
	t.Log(xconv.String(&b))

	slice := []string{"1"}
	fun := func() {}
	t.Log(xconv.String(&slice))
	t.Log(xconv.String(fun))
}

func TestBool(t *testing.T) {
	a := float32(0)
	t.Log(xconv.Bool(a))
	t.Log(xconv.Bool(&a))
}

func TestDuration(t *testing.T) {
	t.Log(xconv.Duration("3d5m4h0.4d"))
}

func TestStrings(t *testing.T) {
	any := []int64{1, 2, 3, 4}
	t.Log(xconv.Strings(any))
}

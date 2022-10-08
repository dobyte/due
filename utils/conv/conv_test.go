package conv_test

import (
	"github.com/dobyte/due/utils/conv"
	"math"
	"math/cmplx"
	"testing"
	"time"
)

func TestInt64(t *testing.T) {
	a := cmplx.Exp(1i*math.Pi) + 20
	t.Log(conv.Int64(time.Now()))
	t.Log(conv.Int64(func() {}))
	t.Log(conv.Int64(&a))
}

func TestString(t *testing.T) {
	t.Log(conv.String(1))
	t.Log(conv.String(int8(1)))

	var a = int64(1)
	var b = 1.1
	var c = &b

	t.Log(conv.String(&a))
	t.Log(*c)
	t.Log(conv.String(&b))

	slice := []string{"1"}
	fun := func() {}
	t.Log(conv.String(&slice))
	t.Log(conv.String(fun))
}

func TestBool(t *testing.T) {
	a := float32(0)
	t.Log(conv.Bool(a))
	t.Log(conv.Bool(&a))
}

func TestDuration(t *testing.T) {
	t.Log(conv.Duration("3d5m4h0.4d"))
}

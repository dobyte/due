package xconv_test

import (
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xtime"
	"math"
	"math/cmplx"
	"testing"
)

func TestInt64(t *testing.T) {
	a := cmplx.Exp(1i*math.Pi) + 20
	t.Log(xconv.Int64(xtime.Now()))
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
	a := []int64{1, 2, 3, 4}
	t.Log(xconv.Strings(a))
}

func TestBytes(t *testing.T) {
	t.Log(xconv.Bytes("1"))
	t.Log(xconv.Int(xconv.String(xconv.Bytes("1"))))
	t.Log(xconv.Bytes(1))
	t.Log(xconv.Int(xconv.Bytes(1)))
	t.Log(xconv.Bytes(uint8(255)))
	t.Log(xconv.Int(xconv.Bytes(uint8(255))))
	t.Log(xconv.Bytes(255))
	t.Log(xconv.Int(xconv.Bytes(255)))
}

func TestJson(t *testing.T) {
	t.Log(xconv.Json("{}"))
	t.Log(xconv.Json(`{"id":1,"name":"fuxiao"}`))
	t.Log(xconv.Json("[]"))
	t.Log(xconv.Json(`[{"id":1,"name":"fuxiao"}]`))
	t.Log(xconv.Json(map[string]interface{}{
		"id":   1,
		"name": "fuxiao",
	}))
	t.Log(xconv.Json([]map[string]interface{}{{
		"id":   1,
		"name": "fuxiao",
	}}))
	t.Log(xconv.Json(struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{
		ID:   1,
		Name: "fuxiao",
	}))
	t.Log(xconv.Json([]struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}{
		{
			ID:   1,
			Name: "fuxiao",
		},
	}))
}

package xrand_test

import (
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/dobyte/due/v2/utils/xrand"
	"testing"
)

func Test_Str(t *testing.T) {
	t.Log(xrand.Str("您好中国AJCKEKD", 5))
}

func Test_Symbols(t *testing.T) {
	t.Log(xrand.Symbols(5))
}

func Test_Int(t *testing.T) {
	t.Log(xrand.Int(1, 2))
}

func Test_Float32(t *testing.T) {
	t.Log(xrand.Float32(-50, 5))
}

func TestLucky(t *testing.T) {
	t.Log(xrand.Lucky(50.201222))
	t.Log(xrand.Lucky(0.201222))
	t.Log(xrand.Lucky(50))
	t.Log(xrand.Lucky(0))
}

func TestWeight(t *testing.T) {
	i := xrand.Weight([]interface{}{
		50,
		20.3,
		39.7,
	}, func(v interface{}) float64 {
		return xconv.Float64(v)
	})

	t.Log(i)
}

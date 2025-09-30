package xmath_test

import (
	"math"
	"testing"

	"github.com/dobyte/due/v2/utils/xmath"
)

func TestFloor(t *testing.T) {
	f := math.Pi

	t.Log(f)
	t.Log(xmath.Floor(f, 2))
}

func TestCeil(t *testing.T) {
	f := math.Pi

	t.Log(f)
	t.Log(xmath.Ceil(f, 2))
}

func TestRound(t *testing.T) {
	f := math.Pi

	t.Log(f)
	t.Log(xmath.Round(f, 2))
}

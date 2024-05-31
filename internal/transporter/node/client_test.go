package node_test

import (
	"math"
	"sync/atomic"
	"testing"
)

var idx uint64

func TestNewClient(t *testing.T) {
	atomic.AddUint64(&idx, math.MaxUint64)

	t.Log(atomic.AddUint64(&idx, 1))
	t.Log(atomic.AddUint64(&idx, 1))
}

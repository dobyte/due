package internal_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dobyte/due/v2/log/internal"
)

func TestString(t *testing.T) {
	v1 := "hello"
	v2 := &v1
	v3 := 123
	v4 := &v3
	v5 := true
	v6 := &v5
	v7 := float32(1.23)
	v8 := &v7
	v9 := time.Now()
	v10 := &v9
	v11 := time.Time{}
	v12 := &v11
	v13 := []byte("hello")
	v14 := &v13

	t.Log(internal.String(v1) == fmt.Sprintf("%v", v1))
	t.Log(internal.String(v2) == fmt.Sprintf("%v", v2))
	t.Log(internal.String(v3) == fmt.Sprintf("%v", v3))
	t.Log(internal.String(v4) == fmt.Sprintf("%v", v4))
	t.Log(internal.String(v5) == fmt.Sprintf("%v", v5))
	t.Log(internal.String(v6) == fmt.Sprintf("%v", v6))
	t.Log(internal.String(v7) == fmt.Sprintf("%v", v7))
	t.Log(internal.String(v8) == fmt.Sprintf("%v", v8))
	t.Log(internal.String(v9) == fmt.Sprintf("%v", v9))
	t.Log(internal.String(v10) == fmt.Sprintf("%v", v10))
	t.Log(internal.String(v11) == fmt.Sprintf("%v", v11))
	t.Log(internal.String(v12) == fmt.Sprintf("%v", v12))
	t.Log(internal.String(v13) == fmt.Sprintf("%v", v13))
	t.Log(internal.String(v14) == fmt.Sprintf("%v", v14))

	t.Log(internal.String(v13))
	t.Log(internal.String(v14))
	t.Log(fmt.Sprintf("%v", v13))
	t.Log(fmt.Sprintf("%v", v14))
}

func BenchmarkString(b *testing.B) {
	v1 := "hello"
	// v2 := &v1
	// v3 := 123
	// v4 := &v3
	// v5 := true
	// v6 := &v5
	// v7 := float32(1.23)
	// v8 := &v7
	// v9 := time.Now()
	// v10 := &v9
	// v11 := time.Time{}
	// v12 := &v11
	// v13 := []byte("hello")
	// v14 := &v13

	b.Run("String", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			internal.String(v1)
			// internal.String(v2)
			// internal.String(v3)
			// internal.String(v4)
			// internal.String(v5)
			// internal.String(v6)
			// internal.String(v7)
			// internal.String(v8)
			// internal.String(v9)
			// internal.String(v10)
			// internal.String(v11)
			// internal.String(v12)
			// internal.String(v13)
			// internal.String(v14)
		}
	})
}

func Benchmark(b *testing.B) {
	// v1 := "hello"
	// v2 := &v1
	// v3 := 123
	// v4 := &v3
	// v5 := true
	// v6 := &v5
	// v7 := float32(1.23)
	// v8 := &v7
	// v9 := time.Now()
	// v10 := &v9
	// v11 := time.Time{}
	// v12 := &v11
	v13 := []byte("hello")
	// v14 := &v13

	b.Run("String", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// internal.String(v1)
			// internal.String(v2)
			// internal.String(v3)
			// internal.String(v4)
			// internal.String(v5)
			// internal.String(v6)
			// internal.String(v7)
			// internal.String(v8)
			// internal.String(v9)
			// internal.String(v10)
			// internal.String(v11)
			// internal.String(v12)
			internal.String(v13)
			// internal.String(v14)
		}
	})

	b.Run("Sprintf", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// fmt.Sprintf("%v", v1)
			// fmt.Sprintf("%v", v2)
			// fmt.Sprintf("%v", v3)
			// fmt.Sprintf("%v", v4)
			// fmt.Sprintf("%v", v5)
			// fmt.Sprintf("%v", v6)
			// fmt.Sprintf("%v", v7)
			// fmt.Sprintf("%v", v8)
			// fmt.Sprintf("%v", v9)
			// fmt.Sprintf("%v", v10)
			// fmt.Sprintf("%v", v11)
			// fmt.Sprintf("%v", v12)
			fmt.Sprintf("%v", v13)
			// fmt.Sprintf("%v", v14)
		}
	})
}

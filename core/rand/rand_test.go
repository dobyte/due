package rand_test

import (
	"math"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/v2/core/rand"
)

// generators 返回所有需要测试的随机数生成器实例
// 覆盖 PseudoRandom、TrueRandom 以及各自的并发安全模式
func generators() map[string]*rand.Rand {
	return map[string]*rand.Rand{
		"pseudo":      rand.NewRand(&rand.PseudoRandom{}),
		"true":        rand.NewRand(&rand.TrueRandom{}),
		"pseudo_safe": rand.NewRand(&rand.PseudoRandom{}, true),
		"true_safe":   rand.NewRand(&rand.TrueRandom{}, true),
	}
}

// runForAll 针对每个生成器实例运行相同的断言逻辑
func runForAll(t *testing.T, fn func(t *testing.T, r *rand.Rand)) {
	t.Helper()
	for name, r := range generators() {
		t.Run(name, func(t *testing.T) {
			fn(t, r)
		})
	}
}

// mustPanic 断言函数执行时必然触发 panic
func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Errorf("期望 panic，但未发生")
		}
	}()
	fn()
}

func TestInt64(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Int64(); v < 0 {
				t.Fatalf("Int64 返回负数: %d", v)
			}
		}
	})
}

func TestUint32(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			_ = r.Uint32()
		}
	})
}

func TestUint64(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			_ = r.Uint64()
		}
	})
}

func TestInt32(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Int32(); v < 0 {
				t.Fatalf("Int32 返回负数: %d", v)
			}
		}
	})
}

func TestInt(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Int(); v < 0 {
				t.Fatalf("Int 返回负数: %d", v)
			}
		}
	})
}

func TestUint(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			_ = r.Uint()
		}
	})
}

func TestInt64N(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Int64N(100); v < 0 || v >= 100 {
				t.Fatalf("Int64N(100) 越界: %d", v)
			}
		}
		if v := r.Int64N(1); v != 0 {
			t.Fatalf("Int64N(1) 应为 0，实际: %d", v)
		}
		mustPanic(t, func() { r.Int64N(0) })
	})
}

func TestUint64N(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Uint64N(100); v >= 100 {
				t.Fatalf("Uint64N(100) 越界: %d", v)
			}
		}
		mustPanic(t, func() { r.Uint64N(0) })
	})
}

func TestInt32N(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Int32N(100); v < 0 || v >= 100 {
				t.Fatalf("Int32N(100) 越界: %d", v)
			}
		}
		mustPanic(t, func() { r.Int32N(0) })
	})
}

func TestUint32N(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Uint32N(100); v >= 100 {
				t.Fatalf("Uint32N(100) 越界: %d", v)
			}
		}
		mustPanic(t, func() { r.Uint32N(0) })
	})
}

func TestIntN(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.IntN(100); v < 0 || v >= 100 {
				t.Fatalf("IntN(100) 越界: %d", v)
			}
		}
		if v := r.IntN(1); v != 0 {
			t.Fatalf("IntN(1) 应为 0，实际: %d", v)
		}
		mustPanic(t, func() { r.IntN(0) })
	})
}

func TestUintN(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.UintN(100); v >= 100 {
				t.Fatalf("UintN(100) 越界: %d", v)
			}
		}
		mustPanic(t, func() { r.UintN(0) })
	})
}

func TestFloat64(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Float64(); v < 0 || v >= 1 {
				t.Fatalf("Float64 越界: %f", v)
			}
		}
	})
}

func TestFloat32(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Float32(); v < 0 || v >= 1 {
				t.Fatalf("Float32 越界: %f", v)
			}
		}
	})
}

func TestPerm(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		p := r.Perm(10)
		if len(p) != 10 {
			t.Fatalf("Perm(10) 长度错误: %d", len(p))
		}
		seen := make([]bool, 10)
		for _, v := range p {
			if v < 0 || v >= 10 {
				t.Fatalf("Perm 元素越界: %d", v)
			}
			if seen[v] {
				t.Fatalf("Perm 元素重复: %d", v)
			}
			seen[v] = true
		}
	})
}

func TestExpFloat64(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.ExpFloat64(); v <= 0 {
				t.Fatalf("ExpFloat64 应为正数: %f", v)
			}
		}
	})
}

func TestNormFloat64(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			v := r.NormFloat64()
			if math.IsNaN(v) || math.IsInf(v, 0) {
				t.Fatalf("NormFloat64 返回非法值: %f", v)
			}
		}
	})
}

func TestStr(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		s := r.Str(rand.LetterSeed, 20)
		if len(s) != 20 {
			t.Fatalf("Str 长度错误: %d", len(s))
		}
		for _, c := range s {
			if !strings.ContainsRune(rand.LetterSeed, c) {
				t.Fatalf("Str 出现种子外字符: %q", c)
			}
		}

		if s = r.Str(rand.LetterSeed, 0); s != "" {
			t.Fatalf("Str(0) 应为空字符串，实际: %q", s)
		}
		if s = r.Str("", 10); s != "" {
			t.Fatalf("空种子应返回空字符串，实际: %q", s)
		}
	})
}

func TestLetters(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		s := r.Letters(20)
		if len(s) != 20 {
			t.Fatalf("Letters 长度错误: %d", len(s))
		}
		for _, c := range s {
			if !strings.ContainsRune(rand.LetterSeed, c) {
				t.Fatalf("Letters 出现字母表外字符: %q", c)
			}
		}
	})
}

func TestDigits(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		// 默认首位不允许为 0
		s := r.Digits(6)
		if len(s) != 6 {
			t.Fatalf("Digits 长度错误: %d", len(s))
		}
		if s[0] == '0' {
			t.Fatalf("Digits 首位不应为 0: %q", s)
		}
		for _, c := range s {
			if !strings.ContainsRune(rand.DigitSeed, c) {
				t.Fatalf("Digits 出现数字外字符: %q", c)
			}
		}

		// 允许首位为 0
		s = r.Digits(6, true)
		if len(s) != 6 {
			t.Fatalf("Digits(6,true) 长度错误: %d", len(s))
		}
		for _, c := range s {
			if !strings.ContainsRune(rand.DigitSeed, c) {
				t.Fatalf("Digits(6,true) 出现数字外字符: %q", c)
			}
		}

		// 长度为 1 时首位不应为 0
		if s = r.Digits(1); len(s) != 1 || s[0] == '0' {
			t.Fatalf("Digits(1) 异常: %q", s)
		}

		// 非正长度返回空字符串
		if s = r.Digits(0); s != "" {
			t.Fatalf("Digits(0) 应为空字符串，实际: %q", s)
		}
	})
}

func TestSymbols(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		s := r.Symbols(20)
		if len(s) != 20 {
			t.Fatalf("Symbols 长度错误: %d", len(s))
		}
		for _, c := range s {
			if !strings.ContainsRune(rand.SymbolSeed, c) {
				t.Fatalf("Symbols 出现特殊字符表外字符: %q", c)
			}
		}
	})
}

func TestIntR(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.IntR(10, 20); v < 10 || v > 20 {
				t.Fatalf("IntR(10,20) 越界: %d", v)
			}
		}
		// min > max 时自动交换
		for i := 0; i < 1000; i++ {
			if v := r.IntR(20, 10); v < 10 || v > 20 {
				t.Fatalf("IntR(20,10) 越界: %d", v)
			}
		}
		// min == max 时直接返回
		if v := r.IntR(5, 5); v != 5 {
			t.Fatalf("IntR(5,5) 应为 5，实际: %d", v)
		}
	})
}

func TestInt32R(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Int32R(10, 20); v < 10 || v > 20 {
				t.Fatalf("Int32R(10,20) 越界: %d", v)
			}
		}
		if v := r.Int32R(5, 5); v != 5 {
			t.Fatalf("Int32R(5,5) 应为 5，实际: %d", v)
		}
	})
}

func TestInt64R(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Int64R(10, 20); v < 10 || v > 20 {
				t.Fatalf("Int64R(10,20) 越界: %d", v)
			}
		}
		if v := r.Int64R(5, 5); v != 5 {
			t.Fatalf("Int64R(5,5) 应为 5，实际: %d", v)
		}
	})
}

func TestFloat32R(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Float32R(1.0, 2.0); v < 1.0 || v >= 2.0 {
				t.Fatalf("Float32R(1,2) 越界: %f", v)
			}
		}
		if v := r.Float32R(1.5, 1.5); v != 1.5 {
			t.Fatalf("Float32R(1.5,1.5) 应为 1.5，实际: %f", v)
		}
	})
}

func TestFloat64R(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Float64R(1.0, 2.0); v < 1.0 || v >= 2.0 {
				t.Fatalf("Float64R(1,2) 越界: %f", v)
			}
		}
		if v := r.Float64R(1.5, 1.5); v != 1.5 {
			t.Fatalf("Float64R(1.5,1.5) 应为 1.5，实际: %f", v)
		}
	})
}

func TestDuration(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			if v := r.Duration(time.Second, 2*time.Second); v < time.Second || v > 2*time.Second {
				t.Fatalf("Duration 越界: %v", v)
			}
		}
	})
}

func TestLucky(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		if r.Lucky(0) {
			t.Fatal("Lucky(0) 应为 false")
		}
		if r.Lucky(-1) {
			t.Fatal("Lucky(-1) 应为 false")
		}
		if !r.Lucky(100) {
			t.Fatal("Lucky(100) 应为 true")
		}
		if !r.Lucky(1000) {
			t.Fatal("Lucky(1000) 应为 true")
		}
		// 概率落在(0,100)区间，多次调用不应 panic
		for i := 0; i < 1000; i++ {
			_ = r.Lucky(50)
		}
	})
}

func TestWeight(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		for i := 0; i < 1000; i++ {
			idx, v, ok := r.Weight(func(x int) float64 { return float64(x) }, 1, 2, 3)
			if !ok {
				t.Fatal("Weight 应命中")
			}
			if idx < 0 || idx >= 3 {
				t.Fatalf("Weight 下标越界: %d", idx)
			}
			if v != 1 && v != 2 && v != 3 {
				t.Fatalf("Weight 返回元素异常: %d", v)
			}
		}

		// 空列表
		if _, _, ok := r.Weight(func(x int) float64 { return 1 }); ok {
			t.Fatal("空列表 Weight 应返回 false")
		}

		// 权重合计为 0
		if idx, _, ok := r.Weight(func(x int) float64 { return 0 }, 1, 2, 3); ok || idx != -1 {
			t.Fatalf("全零权重应返回 false 且下标为 -1，实际 ok=%v idx=%d", ok, idx)
		}
	})
}

func TestShuffle(t *testing.T) {
	runForAll(t, func(t *testing.T, r *rand.Rand) {
		list := []int{1, 2, 3, 4, 5}
		orig := append([]int(nil), list...)
		r.Shuffle(list)
		if len(list) != len(orig) {
			t.Fatalf("Shuffle 后长度变化: %d", len(list))
		}
		sort.Ints(list)
		sort.Ints(orig)
		for i := range orig {
			if list[i] != orig[i] {
				t.Fatalf("Shuffle 后元素集合发生变化: %v", list)
			}
		}
	})
}

func TestGenericN(t *testing.T) {
	t.Run("pseudo", func(t *testing.T) {
		r := &rand.PseudoRandom{}
		for i := 0; i < 1000; i++ {
			if v := r.N(10); v < 0 || v >= 10 {
				t.Fatalf("N(10) 越界: %d", v)
			}
		}
		mustPanic(t, func() { r.N(0) })
	})
	t.Run("true", func(t *testing.T) {
		r := &rand.TrueRandom{}
		for i := 0; i < 1000; i++ {
			if v := r.N(10); v < 0 || v >= 10 {
				t.Fatalf("N(10) 越界: %d", v)
			}
		}
		mustPanic(t, func() { r.N(0) })
	})
}

func TestConcurrency(t *testing.T) {
	for name, r := range generators() {
		t.Run(name, func(t *testing.T) {
			var wg sync.WaitGroup
			for g := 0; g < 50; g++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := 0; i < 100; i++ {
						r.Int64()
						r.IntN(100)
						r.Uint64()
						r.Float64()
						r.Letters(10)
						r.Digits(6)
						r.IntR(1, 100)
						r.Perm(10)
						r.ExpFloat64()
						r.NormFloat64()
					}
				}()
			}
			wg.Wait()
		})
	}
}

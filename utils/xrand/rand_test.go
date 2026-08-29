package xrand_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/v2/utils/xrand"
)

func TestUint32(t *testing.T) {
	for range 100 {
		_ = xrand.Uint32()
	}
}

func TestUint64(t *testing.T) {
	for range 100 {
		_ = xrand.Uint64()
	}
}

func TestInt32(t *testing.T) {
	for range 100 {
		if got := xrand.Int32(); got < 0 {
			t.Fatalf("Int32() = %d, want >= 0", got)
		}
	}
}

func TestInt(t *testing.T) {
	for range 100 {
		if got := xrand.Int(); got < 0 {
			t.Fatalf("Int() = %d, want >= 0", got)
		}
	}
}

func TestUint(t *testing.T) {
	for range 100 {
		_ = xrand.Uint()
	}
}

func TestInt64N(t *testing.T) {
	for range 1000 {
		if got := xrand.Int64N(100); got < 0 || got >= 100 {
			t.Fatalf("Int64N(100) = %d, want in [0, 100)", got)
		}
	}
}

func TestUint64N(t *testing.T) {
	for range 1000 {
		if got := xrand.Uint64N(100); got >= 100 {
			t.Fatalf("Uint64N(100) = %d, want in [0, 100)", got)
		}
	}
}

func TestInt32N(t *testing.T) {
	for range 1000 {
		if got := xrand.Int32N(100); got < 0 || got >= 100 {
			t.Fatalf("Int32N(100) = %d, want in [0, 100)", got)
		}
	}
}

func TestUint32N(t *testing.T) {
	for range 1000 {
		if got := xrand.Uint32N(100); got >= 100 {
			t.Fatalf("Uint32N(100) = %d, want in [0, 100)", got)
		}
	}
}

func TestIntN(t *testing.T) {
	for range 1000 {
		if got := xrand.IntN(100); got < 0 || got >= 100 {
			t.Fatalf("IntN(100) = %d, want in [0, 100)", got)
		}
	}
}

func TestUintN(t *testing.T) {
	for range 1000 {
		if got := xrand.UintN(100); got >= 100 {
			t.Fatalf("UintN(100) = %d, want in [0, 100)", got)
		}
	}
}

func TestFloat64(t *testing.T) {
	for range 1000 {
		if got := xrand.Float64(); got < 0 || got >= 1 {
			t.Fatalf("Float64() = %v, want in [0, 1)", got)
		}
	}
}

func TestFloat32(t *testing.T) {
	for range 1000 {
		if got := xrand.Float32(); got < 0 || got >= 1 {
			t.Fatalf("Float32() = %v, want in [0, 1)", got)
		}
	}
}

func TestPerm(t *testing.T) {
	for n := 0; n <= 10; n++ {
		p := xrand.Perm(n)
		if len(p) != n {
			t.Fatalf("Perm(%d) length = %d, want %d", n, len(p), n)
		}

		seen := make(map[int]struct{}, n)
		for _, v := range p {
			if v < 0 || v >= n {
				t.Fatalf("Perm(%d) contains %d, want in [0, %d)", n, v, n)
			}
			seen[v] = struct{}{}
		}

		if len(seen) != n {
			t.Fatalf("Perm(%d) contains duplicates", n)
		}
	}
}

func TestExpFloat64(t *testing.T) {
	for range 1000 {
		got := xrand.ExpFloat64()
		if got < 0 || math.IsInf(got, 0) || math.IsNaN(got) {
			t.Fatalf("ExpFloat64() = %v, want finite non-negative", got)
		}
	}
}

func TestNormFloat64(t *testing.T) {
	for range 1000 {
		got := xrand.NormFloat64()
		if math.IsInf(got, 0) || math.IsNaN(got) {
			t.Fatalf("NormFloat64() = %v, want finite", got)
		}
	}
}

func TestStr(t *testing.T) {
	t.Run("nonPositiveLength", func(t *testing.T) {
		if got := xrand.Str("abc", 0); got != "" {
			t.Fatalf("Str(abc, 0) = %q, want empty", got)
		}
		if got := xrand.Str("abc", -1); got != "" {
			t.Fatalf("Str(abc, -1) = %q, want empty", got)
		}
	})

	t.Run("emptySeed", func(t *testing.T) {
		if got := xrand.Str("", 5); got != "" {
			t.Fatalf("Str(\"\", 5) = %q, want empty", got)
		}
	})

	t.Run("normal", func(t *testing.T) {
		seed := "您好中国ABC"
		for range 100 {
			got := xrand.Str(seed, 10)
			if n := len([]rune(got)); n != 10 {
				t.Fatalf("Str length = %d, want 10", n)
			}
			for _, c := range got {
				if !strings.ContainsRune(seed, c) {
					t.Fatalf("Str contains rune %q not in seed", c)
				}
			}
		}
	})
}

func TestLetters(t *testing.T) {
	if got := xrand.Letters(0); got != "" {
		t.Fatalf("Letters(0) = %q, want empty", got)
	}

	for range 100 {
		got := xrand.Letters(16)
		if len(got) != 16 {
			t.Fatalf("Letters(16) length = %d, want 16", len(got))
		}
		for _, c := range got {
			if !strings.ContainsRune(xrand.LetterSeed, c) {
				t.Fatalf("Letters contains rune %q not in letter seed", c)
			}
		}
	}
}

func TestDigits(t *testing.T) {
	t.Run("nonPositiveLength", func(t *testing.T) {
		if got := xrand.Digits(0); got != "" {
			t.Fatalf("Digits(0) = %q, want empty", got)
		}
		if got := xrand.Digits(-1); got != "" {
			t.Fatalf("Digits(-1) = %q, want empty", got)
		}
	})

	t.Run("hasLeadingZero", func(t *testing.T) {
		for range 100 {
			got := xrand.Digits(8, true)
			if len(got) != 8 {
				t.Fatalf("Digits(8, true) length = %d, want 8", len(got))
			}
			for _, c := range got {
				if !strings.ContainsRune(xrand.DigitSeed, c) {
					t.Fatalf("Digits contains rune %q not in digit seed", c)
				}
			}
		}
	})

	t.Run("lengthOne", func(t *testing.T) {
		for range 100 {
			got := xrand.Digits(1)
			if len(got) != 1 {
				t.Fatalf("Digits(1) length = %d, want 1", len(got))
			}
			if !strings.ContainsRune(xrand.DigitWithoutZeroSeed, []rune(got)[0]) {
				t.Fatalf("Digits(1) = %q, want a digit without zero", got)
			}
		}
	})

	t.Run("noLeadingZero", func(t *testing.T) {
		for range 100 {
			got := xrand.Digits(8)
			if len(got) != 8 {
				t.Fatalf("Digits(8) length = %d, want 8", len(got))
			}

			rs := []rune(got)
			if !strings.ContainsRune(xrand.DigitWithoutZeroSeed, rs[0]) {
				t.Fatalf("Digits(8) first rune %q should not be zero", rs[0])
			}
			for _, c := range rs[1:] {
				if !strings.ContainsRune(xrand.DigitSeed, c) {
					t.Fatalf("Digits contains rune %q not in digit seed", c)
				}
			}
		}
	})
}

func TestSymbols(t *testing.T) {
	if got := xrand.Symbols(0); got != "" {
		t.Fatalf("Symbols(0) = %q, want empty", got)
	}

	for range 100 {
		got := xrand.Symbols(16)
		if len(got) != 16 {
			t.Fatalf("Symbols(16) length = %d, want 16", len(got))
		}
		for _, c := range got {
			if !strings.ContainsRune(xrand.SymbolSeed, c) {
				t.Fatalf("Symbols contains rune %q not in symbol seed", c)
			}
		}
	}
}

func TestIntR(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		if got := xrand.IntR(10, 10); got != 10 {
			t.Fatalf("IntR(10, 10) = %d, want 10", got)
		}
	})

	t.Run("reversed", func(t *testing.T) {
		for range 1000 {
			if got := xrand.IntR(10, -10); got < -10 || got > 10 {
				t.Fatalf("IntR(10, -10) = %d, want in [-10, 10]", got)
			}
		}
	})

	t.Run("normal", func(t *testing.T) {
		for range 1000 {
			if got := xrand.IntR(-10, 10); got < -10 || got > 10 {
				t.Fatalf("IntR(-10, 10) = %d, want in [-10, 10]", got)
			}
		}
	})

	t.Run("fullRange", func(t *testing.T) {
		for range 100 {
			_ = xrand.IntR(math.MinInt, math.MaxInt)
		}
	})
}

func TestInt32R(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		if got := xrand.Int32R(10, 10); got != 10 {
			t.Fatalf("Int32R(10, 10) = %d, want 10", got)
		}
	})

	t.Run("reversed", func(t *testing.T) {
		for range 1000 {
			if got := xrand.Int32R(10, -10); got < -10 || got > 10 {
				t.Fatalf("Int32R(10, -10) = %d, want in [-10, 10]", got)
			}
		}
	})

	t.Run("normal", func(t *testing.T) {
		for range 1000 {
			if got := xrand.Int32R(-10, 10); got < -10 || got > 10 {
				t.Fatalf("Int32R(-10, 10) = %d, want in [-10, 10]", got)
			}
		}
	})

	t.Run("fullRange", func(t *testing.T) {
		for range 100 {
			_ = xrand.Int32R(math.MinInt32, math.MaxInt32)
		}
	})
}

func TestInt64R(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		if got := xrand.Int64R(10, 10); got != 10 {
			t.Fatalf("Int64R(10, 10) = %d, want 10", got)
		}
	})

	t.Run("reversed", func(t *testing.T) {
		for range 1000 {
			if got := xrand.Int64R(10, -10); got < -10 || got > 10 {
				t.Fatalf("Int64R(10, -10) = %d, want in [-10, 10]", got)
			}
		}
	})

	t.Run("normal", func(t *testing.T) {
		for range 1000 {
			if got := xrand.Int64R(-10, 10); got < -10 || got > 10 {
				t.Fatalf("Int64R(-10, 10) = %d, want in [-10, 10]", got)
			}
		}
	})

	t.Run("fullRange", func(t *testing.T) {
		for range 100 {
			_ = xrand.Int64R(math.MinInt64, math.MaxInt64)
		}
	})
}

func TestFloat32R(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		if got := xrand.Float32R(3.14, 3.14); got != 3.14 {
			t.Fatalf("Float32R(3.14, 3.14) = %v, want 3.14", got)
		}
	})

	t.Run("reversed", func(t *testing.T) {
		for range 1000 {
			if got := xrand.Float32R(5, -50); got < -50 || got >= 5 {
				t.Fatalf("Float32R(5, -50) = %v, want in [-50, 5)", got)
			}
		}
	})

	t.Run("normal", func(t *testing.T) {
		for range 1000 {
			if got := xrand.Float32R(-50, 5); got < -50 || got >= 5 {
				t.Fatalf("Float32R(-50, 5) = %v, want in [-50, 5)", got)
			}
		}
	})
}

func TestFloat64R(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		if got := xrand.Float64R(3.14, 3.14); got != 3.14 {
			t.Fatalf("Float64R(3.14, 3.14) = %v, want 3.14", got)
		}
	})

	t.Run("reversed", func(t *testing.T) {
		for range 1000 {
			if got := xrand.Float64R(5, -50); got < -50 || got >= 5 {
				t.Fatalf("Float64R(5, -50) = %v, want in [-50, 5)", got)
			}
		}
	})

	t.Run("normal", func(t *testing.T) {
		for range 1000 {
			if got := xrand.Float64R(-50, 5); got < -50 || got >= 5 {
				t.Fatalf("Float64R(-50, 5) = %v, want in [-50, 5)", got)
			}
		}
	})
}

func TestDuration(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		if got := xrand.Duration(time.Second, time.Second); got != time.Second {
			t.Fatalf("Duration(1s, 1s) = %v, want 1s", got)
		}
	})

	t.Run("reversed", func(t *testing.T) {
		for range 1000 {
			got := xrand.Duration(10*time.Second, time.Second)
			if got < time.Second || got > 10*time.Second {
				t.Fatalf("Duration(10s, 1s) = %v, want in [1s, 10s]", got)
			}
		}
	})

	t.Run("normal", func(t *testing.T) {
		for range 1000 {
			got := xrand.Duration(time.Second, 10*time.Second)
			if got < time.Second || got > 10*time.Second {
				t.Fatalf("Duration(1s, 10s) = %v, want in [1s, 10s]", got)
			}
		}
	})
}

func TestLucky(t *testing.T) {
	t.Run("nonPositiveProbability", func(t *testing.T) {
		if xrand.Lucky(0) {
			t.Fatal("Lucky(0) = true, want false")
		}
		if xrand.Lucky(-1) {
			t.Fatal("Lucky(-1) = true, want false")
		}
	})

	t.Run("nonPositiveBase", func(t *testing.T) {
		if xrand.Lucky(50, 0) {
			t.Fatal("Lucky(50, 0) = true, want false")
		}
		if xrand.Lucky(50, -1) {
			t.Fatal("Lucky(50, -1) = true, want false")
		}
	})

	t.Run("probabilityNotLessThanBase", func(t *testing.T) {
		if !xrand.Lucky(100) {
			t.Fatal("Lucky(100) = false, want true")
		}
		if !xrand.Lucky(50, 50) {
			t.Fatal("Lucky(50, 50) = false, want true")
		}
		if !xrand.Lucky(100, 50) {
			t.Fatal("Lucky(100, 50) = false, want true")
		}
	})

	t.Run("statistical", func(t *testing.T) {
		const (
			total       = 100000
			probability = 50.0
		)

		hits := 0
		for range total {
			if xrand.Lucky(probability) {
				hits++
			}
		}

		if hits == 0 || hits == total {
			t.Fatalf("Lucky(50) hits = %d/%d, want in (0, %d)", hits, total, total)
		}
	})
}

func TestWeight(t *testing.T) {
	toFloat64 := func(v float32) float64 { return float64(v) }

	t.Run("empty", func(t *testing.T) {
		i, v, ok := xrand.Weight(toFloat64)
		if i != -1 {
			t.Fatalf("Weight() index = %d, want -1", i)
		}
		if v != 0 {
			t.Fatalf("Weight() value = %v, want zero value", v)
		}
		if ok {
			t.Fatal("Weight() ok = true, want false")
		}
	})

	t.Run("nonPositiveTotal", func(t *testing.T) {
		i, v, ok := xrand.Weight(toFloat64, float32(0), float32(-5))
		if i != -1 {
			t.Fatalf("Weight() index = %d, want -1", i)
		}
		if v != 0 {
			t.Fatalf("Weight() value = %v, want zero value", v)
		}
		if ok {
			t.Fatal("Weight() ok = true, want false")
		}
	})

	t.Run("mixed", func(t *testing.T) {
		for range 1000 {
			i, v, ok := xrand.Weight(toFloat64, float32(-5), float32(0), float32(10), float32(20))
			if !ok {
				t.Fatal("Weight() ok = false, want true")
			}
			if i != 2 && i != 3 {
				t.Fatalf("Weight() index = %d, want 2 or 3", i)
			}
			if v != 10 && v != 20 {
				t.Fatalf("Weight() value = %v, want 10 or 20", v)
			}
		}
	})
}

func TestShuffle(t *testing.T) {
	list := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	xrand.Shuffle(list)

	if len(list) != 10 {
		t.Fatalf("Shuffle length = %d, want 10", len(list))
	}

	seen := make(map[int]struct{}, len(list))
	for _, v := range list {
		seen[v] = struct{}{}
	}
	if len(seen) != 10 {
		t.Fatalf("Shuffle lost or duplicated elements: %v", list)
	}
}

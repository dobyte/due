package xmath_test

import (
	"math"
	"testing"

	"github.com/dobyte/due/v2/utils/xmath"
)

func TestFloor(t *testing.T) {
	tests := []struct {
		name string
		f    float64
		n    []int
		want float64
	}{
		{"默认保留0位小数", 3.14159, nil, 3},
		{"保留2位小数", 3.14159, []int{2}, 3.14},
		{"负数向下取整", -3.14159, []int{2}, -3.15},
		{"小数位不足补零", 3.1, []int{2}, 3.1},
		{"边界值不四舍五入", 1.005, []int{2}, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xmath.Floor(tt.f, tt.n...); !almostEqual(got, tt.want) {
				t.Errorf("Floor(%v, %v) = %v, want %v", tt.f, tt.n, got, tt.want)
			}
		})
	}
}

func TestCeil(t *testing.T) {
	tests := []struct {
		name string
		f    float64
		n    []int
		want float64
	}{
		{"默认保留0位小数", 3.14159, nil, 4},
		{"保留2位小数", 3.14159, []int{2}, 3.15},
		{"负数向上取整", -3.14159, []int{2}, -3.14},
		{"尾数恰好进位", 1.001, []int{2}, 1.01},
		{"浮点误差不误进位", 1.10, []int{2}, 1.1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xmath.Ceil(tt.f, tt.n...); !almostEqual(got, tt.want) {
				t.Errorf("Ceil(%v, %v) = %v, want %v", tt.f, tt.n, got, tt.want)
			}
		})
	}
}

func TestRound(t *testing.T) {
	tests := []struct {
		name string
		f    float64
		n    []int
		want float64
	}{
		{"默认保留0位小数", 3.14159, nil, 3},
		{"保留2位小数", 3.14159, []int{2}, 3.14},
		{"正数四舍五入进位", 1.005, []int{2}, 1.01},
		{"浮点误差修正", 2.675, []int{2}, 2.68},
		{"负数四舍五入进位", -1.005, []int{2}, -1.01},
		{"正数舍去", 1.004, []int{2}, 1.0},
		{"半数远离零取整", 2.5, nil, 3},
		{"负数半数远离零取整", -2.5, nil, -3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xmath.Round(tt.f, tt.n...); !almostEqual(got, tt.want) {
				t.Errorf("Round(%v, %v) = %v, want %v", tt.f, tt.n, got, tt.want)
			}
		})
	}
}

// almostEqual 判断两个浮点数是否在可接受的误差范围内相等
func almostEqual(got, want float64) bool {
	return math.Abs(got-want) < 1e-9
}

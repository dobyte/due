package xmath

import "math"

// Floor 舍去取整保留n位小数
func Floor(f float64, n ...int) float64 {
	s := float64(1)

	if len(n) > 0 {
		s = math.Pow10(n[0])
	}

	return math.Floor(f*s) / s
}

// Ceil 进一取整保留n位小数
func Ceil(f float64, n ...int) float64 {
	s := float64(1)

	if len(n) > 0 {
		s = math.Pow10(n[0])
	}

	return math.Ceil(f*s) / s
}

// Round 四舍五入保留n位小数
func Round(f float64, n ...int) float64 {
	s := float64(1)

	if len(n) > 0 {
		s = math.Pow10(n[0])
	}

	return math.Round(f*s) / s
}

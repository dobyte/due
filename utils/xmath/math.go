package xmath

import "math"

// epsilon 用于修正二进制浮点数无法精确表示十进制小数所产生的误差
// 例如 3.14*100 在浮点运算中可能得到 313.99999999999994，
// 通过加减该极小量可将其修正为 314，避免取整结果偏差
const epsilon = 1e-9

// Floor 向下取整，并保留 n 位小数
// @param f float64 待处理的浮点数
// @param n ...int 可选的小数位数，默认保留 0 位小数
// @return @1 float64 向下取整后的结果
func Floor(f float64, n ...int) float64 {
	s := float64(1)

	if len(n) > 0 {
		s = math.Pow10(n[0])
	}

	// 先放大 s 倍取整，再缩小 s 倍，从而实现按小数位取整
	return math.Floor(f*s+epsilon) / s
}

// Ceil 向上取整，并保留 n 位小数
// @param f float64 待处理的浮点数
// @param n ...int 可选的小数位数，默认保留 0 位小数
// @return @1 float64 向上取整后的结果
func Ceil(f float64, n ...int) float64 {
	s := float64(1)

	if len(n) > 0 {
		s = math.Pow10(n[0])
	}

	// 先放大 s 倍取整，再缩小 s 倍，从而实现按小数位取整
	return math.Ceil(f*s-epsilon) / s
}

// Round 四舍五入，并保留 n 位小数
// 采用“半数远离零”规则，例如 Round(-2.5) 返回 -3
// @param f float64 待处理的浮点数
// @param n ...int 可选的小数位数，默认保留 0 位小数
// @return @1 float64 四舍五入后的结果
func Round(f float64, n ...int) float64 {
	s := float64(1)

	if len(n) > 0 {
		s = math.Pow10(n[0])
	}

	// 先放大 s 倍取整，再缩小 s 倍；Copysign 按符号向远离零方向微调以修正浮点误差
	return math.Round(f*s+math.Copysign(epsilon, f*s)) / s
}

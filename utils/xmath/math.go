package xmath

import "math"

// epsilon corrects the error caused by the inability of binary floating-point numbers to represent
// decimal fractions exactly. For example, 3.14*100 may evaluate to 313.99999999999994; adding or
// subtracting this tiny amount corrects it to 314 and avoids a rounding deviation.
const epsilon = 1e-9

// Floor rounds f down to n decimal places. When n is omitted it defaults to 0.
func Floor(f float64, n ...int) float64 {
	s := float64(1)

	if len(n) > 0 {
		s = math.Pow10(n[0])
	}

	// Scale up by s, round, then scale back down to round to the given decimal place.
	return math.Floor(f*s+epsilon) / s
}

// Ceil rounds f up to n decimal places. When n is omitted it defaults to 0.
func Ceil(f float64, n ...int) float64 {
	s := float64(1)

	if len(n) > 0 {
		s = math.Pow10(n[0])
	}

	// Scale up by s, round, then scale back down to round to the given decimal place.
	return math.Ceil(f*s-epsilon) / s
}

// Round rounds f to n decimal places, using the "round half away from zero" rule, so that
// Round(-2.5) returns -3. When n is omitted it defaults to 0.
func Round(f float64, n ...int) float64 {
	s := float64(1)

	if len(n) > 0 {
		s = math.Pow10(n[0])
	}

	// Scale up by s, round, then scale back down; Copysign nudges the value away from zero to
	// correct the floating-point error.
	return math.Round(f*s+math.Copysign(epsilon, f*s)) / s
}

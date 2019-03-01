package utils

// Golang standard library does not support comparing two integers, only float64 comparisons
// MinInt find the smaller integer of the two input integers
func MinInt(a int, b int) int {
	if a >= b {
		return b
	}
	return a
}

// Golang standard library does not support comparing two integers, only float64 comparisons
// MaxInt find the larger integer of the two input integers
func MaxInt(a int, b int) int {
	if a >= b {
		return a
	}
	return b
}

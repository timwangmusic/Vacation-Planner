package utils

// MinInt find the smaller integer of the two input integers
func MinInt(a int, b int) int {
	if a >= b {
		return b
	}
	return a
}

// MaxInt find the larger integer of the two input integers
func MaxInt(a int, b int) int {
	if a >= b {
		return a
	}
	return b
}

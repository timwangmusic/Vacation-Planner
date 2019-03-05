package utils

import (
	"errors"
	"math"
)

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

// refs: https://bit.ly/2GWSlAC
// FindCenter calculates centroid given a set of points with geo coordinates
func FindCenter(points [][]float64) ([]float64, error){
	if len(points) == 0{
		return []float64{}, errors.New("invalid input, number of points cannot be zero")
	}
	if len(points) == 1{
		return points[0], nil
	}
	var x, y, z float64
	for _, point := range points{
		lat := point[0] * math.Pi / 180
		lng := point[1] * math.Pi / 180

		x += math.Cos(lat) * math.Cos(lng)
		y += math.Cos(lat) * math.Sin(lng)
		z += math.Sin(lat)
	}

	numPoints := float64(len(points))
	// calculate average
	x = x / numPoints
	y = y / numPoints
	z = z / numPoints

	centralLng := math.Atan2(y, x) * 180 / math.Pi
	centralLat := math.Atan2(z, math.Sqrt(x * x + y * y)) * 180 / math.Pi
	return []float64{centralLat, centralLng}, nil
}

// do not consider duplication in input number list
func Permutations(nums []int, res *[][]int, start int){
	if start == len(nums){
		tmp := make([]int, len(nums))
		copy(tmp, nums)
		*res = append(*res, tmp)
		return
	}
	for i:=start; i < len(nums); i++{
		swap(&nums, start, i)
		Permutations(nums, res, start+1)
		swap(&nums, i, start)
	}
}

func swap(nums *[]int, i int, j int){
	tmp := (*nums)[i]
	(*nums)[i] = (*nums)[j]
	(*nums)[j] = tmp
}

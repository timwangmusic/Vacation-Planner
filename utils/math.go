package utils

// Permutations does not consider duplication in input number list
func Permutations(nums []int, res *[][]int, start int) {
	if start == len(nums) {
		tmp := make([]int, len(nums))
		copy(tmp, nums)
		*res = append(*res, tmp)
		return
	}
	for i := start; i < len(nums); i++ {
		swap(&nums, start, i)
		Permutations(nums, res, start+1)
		swap(&nums, i, start)
	}
}

func swap(nums *[]int, i int, j int) {
	tmp := (*nums)[i]
	(*nums)[i] = (*nums)[j]
	(*nums)[j] = tmp
}

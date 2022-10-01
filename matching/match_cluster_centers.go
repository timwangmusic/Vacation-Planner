package matching

import (
	"errors"
	"github.com/weihesdlegend/Vacation-planner/utils"
	"math"
)

type clusterCenterPair struct {
	EateryIdx int
	VisitIdx  int
}

func MatchClusterCenters(eClusterCenters [][]float64, vClusterCenters [][]float64) (pairs []clusterCenterPair, err error) {
	if len(eClusterCenters) != len(vClusterCenters) {
		return []clusterCenterPair{}, errors.New("number of eatery clusters and visit clusters must be the same")
	}

	numClusters := len(eClusterCenters)
	indexes := make([]int, numClusters)
	for i := 0; i < numClusters; i++ {
		indexes[i] = i
	}

	var permutations [][]int
	utils.Permutations(indexes, &permutations, 0)

	score := math.Inf(1) // positive infinity

	var finalPermutation []int
	for _, permutation := range permutations {
		pScore := calculateScore(eClusterCenters, vClusterCenters, permutation)
		if pScore < score {
			score = pScore
			finalPermutation = permutation
		}
	}

	for k, index := range finalPermutation {
		pairs = append(pairs, clusterCenterPair{k, index})
	}

	return
}

func calculateScore(eClusterCenters [][]float64, vClusterCenters [][]float64, permutation []int) float64 {
	score := 0.0
	for k, index := range permutation {
		score += clusterDistance(eClusterCenters[k], vClusterCenters[index])
	}
	return score
}

func clusterDistance(clusterA []float64, clusterB []float64) float64 {
	return utils.HaversineDist(clusterA, clusterB)
}

package iowrappers

import (
	"sort"

	"github.com/weihesdlegend/Vacation-planner/POI"
	"github.com/weihesdlegend/Vacation-planner/utils"
)

// SortPlacesByDistance orders places ascending by distance from (lat, lng).
//
// Callers that truncate a candidate list to a limit MUST sort first. Only the Redis
// cache path returns places in distance order; the fresh path appends one place type's
// results after another in Google prominence order, so an unsorted slice[:limit] drops
// the last place types wholesale and can rank a 3km result above a 250m one.
//
// The sort is stable so places at equal distance keep their existing relative order.
func SortPlacesByDistance(places []POI.Place, lat, lng float64) {
	origin := []float64{lat, lng}
	dist := make(map[int]float64, len(places))
	for i := range places {
		loc := places[i].GetLocation()
		dist[i] = utils.HaversineDist(origin, []float64{loc.Latitude, loc.Longitude})
	}
	idx := make([]int, len(places))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool { return dist[idx[a]] < dist[idx[b]] })

	sorted := make([]POI.Place, len(places))
	for newPos, oldPos := range idx {
		sorted[newPos] = places[oldPos]
	}
	copy(places, sorted)
}

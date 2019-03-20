package pathfind

import (
	"math"
	"reflect"
	"testing"
)

func TestAStarFindPath(t *testing.T) {
	t.Parallel()

	type coord struct {
		x, y float64
	}
	type graph map[coord]map[coord]float64

	testCases := map[string]struct {
		inGraph      graph // origin at 0,0; goal at 10,10
		expectedPath []coord
	}{
		"no-branches": {
			inGraph: graph{
				{0, 0}: map[coord]float64{
					{3, 3}: 4.242640687119285,
				},
				{3, 3}: map[coord]float64{
					{6, 6}: 4.242640687119285,
				},
				{6, 6}: map[coord]float64{
					{9, 9}: 4.242640687119285,
				},
				{9, 9}: map[coord]float64{
					{10, 10}: 1.4142135623730951,
				},
			},
			expectedPath: []coord{
				{0, 0},
				{3, 3},
				{6, 6},
				{9, 9},
				{10, 10},
			},
		},
		"crows-path-vs-manhattan": {
			inGraph: graph{
				{0, 0}: map[coord]float64{
					{3, 3}:   4.242640687119285,
					{2.5, 0}: 2.5,
				},
				// crow's path
				{3, 3}: map[coord]float64{
					{6, 6}: 4.242640687119285,
				},
				{6, 6}: map[coord]float64{
					{9, 9}: 4.242640687119285,
				},
				{9, 9}: map[coord]float64{
					{10, 10}: 1.4142135623730951,
				},
				// manhattan
				{2.5, 0}: map[coord]float64{
					{5.0, 0}: 2.5,
				},
				{5.0, 0}: map[coord]float64{
					{7.5, 0}: 2.5,
				},
				{7.5, 0}: map[coord]float64{
					{10.0, 0}: 2.5,
				},
				{10.0, 0}: map[coord]float64{
					{10.0, 2.5}: 2.5,
				},
				{10.0, 0}: map[coord]float64{
					{10.0, 5.0}: 2.5,
				},
				{10.0, 0}: map[coord]float64{
					{10.0, 7.5}: 2.5,
				},
				{10.0, 0}: map[coord]float64{
					{10.0, 10.0}: 2.5,
				},
			},
			expectedPath: []coord{
				{0, 0},
				{3, 3},
				{6, 6},
				{9, 9},
				{10, 10},
			},
		},
		"obvious-branch-not-cheapest-because-obstacle": {
			// graph includes an obstacle you must go around, between 6,6 and 9,9
			// which makes a direct-cutoff from 3,3 -> 6,9 faster than going
			// through 6,6 -> 6,9
			inGraph: graph{
				{0, 0}: map[coord]float64{
					{3, 3}: 4.242640687119285,
				},
				{3, 3}: map[coord]float64{
					{6, 6}: 4.242640687119285,
					{9, 6}: 6.708203932499369,
				},
				{6, 6}: map[coord]float64{
					{9, 6}: 3.0,
				},
				{9, 6}: map[coord]float64{
					{9, 9}: 3.0,
				},
				{9, 9}: map[coord]float64{
					{10, 10}: 1.4142135623730951,
				},
				{-3, 0}: map[coord]float64{
					{10, 10}: 1.0,
				},
			},
			expectedPath: []coord{
				{0, 0},
				{3, 3},
				{9, 6},
				{9, 9},
				{10, 10},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			coster := func(src, dst interface{}) float64 {
				srcC := src.(coord)
				dstC := dst.(coord)
				subMap, found := tc.inGraph[srcC]
				if !found {
					return math.MaxFloat64
				}
				cost, found := subMap[dstC]
				if !found {
					return math.MaxFloat64
				}
				return cost
			}
			isGoal := func(n interface{}) bool {
				nC := n.(coord)
				return nC.x == 10.0 && nC.y == 10.0
			}
			nodeGen := func(src interface{}) []interface{} {
				srcC := src.(coord)
				subMap, found := tc.inGraph[srcC]
				if !found {
					return nil
				}
				neighbors := make([]interface{}, 0, len(subMap))
				for neighbor := range subMap {
					neighbors = append(neighbors, neighbor)
				}
				return neighbors
			}
			estimator := func(src interface{}) float64 {
				srcC := src.(coord)
				dstC := coord{10.0, 10.0}
				// euclidean distance to {10.0, 10.0}
				return math.Sqrt(math.Pow(dstC.x-srcC.x, 2) + math.Pow(dstC.y-srcC.y, 2))
			}

			cameFrom, _, final := AStarFindPath(coord{0, 0}, coster, estimator, isGoal, nodeGen)

			// Rebuild the path from final to origin
			var path []coord
			k := final.(coord)
			for {
				path = append(path, k)
				var found bool
				k, found = cameFrom[k].(coord)
				if !found {
					break
				}
			}
			// Reverse the path so it's origin-first
			// see: https://github.com/golang/go/wiki/SliceTricks#reversing
			for i := len(path)/2 - 1; i >= 0; i-- {
				opp := len(path) - 1 - i
				path[i], path[opp] = path[opp], path[i]
			}

			if !reflect.DeepEqual(path, tc.expectedPath) {
				t.Errorf("expected %v, got %v", tc.expectedPath, path)
			}
		})
	}
}

package pathfind

import (
	"math"
	"reflect"
	"testing"
)

func TestDijkstraFindPath(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		inMap         map[string]map[string]float64
		maxExpansions int
		expectedPath  []string
	}{
		"simple": {
			inMap: map[string]map[string]float64{
				"start": {
					"1a": 1.0,
					"1b": 1.0,
				},
				"1a": {
					"2a": 1.0,
				},
				"1b": {
					"2b": 1.0,
				},
				"2a": {
					"goal": 0.5,
				},
				"2b": {
					"goal": 1.0,
				},
			},
			maxExpansions: -1,
			expectedPath:  []string{"start", "1a", "2a", "goal"},
		},
		"max-expansions-hit": {
			inMap: map[string]map[string]float64{
				"start": {
					"1a": 1.0,
					"1b": 1.0,
				},
				"1a": {
					"2a": 1.0,
				},
				"1b": {
					"2b": 1.0,
				},
				"2a": {
					"goal": 0.5,
				},
				"2b": {
					"goal": 1.0,
				},
			},
			maxExpansions: 5,
			expectedPath:  nil,
		},
		"complex": {
			inMap: map[string]map[string]float64{
				"start": {
					"1a": 1.0,
					"1b": 0.5,
				},
				"1a": {
					"2a": 0.5,
					"2b": 0.5,
					"2c": 1.25,
				},
				"1b": {
					"2a": 2.0,
					"2b": 1.25,
					"2c": 0.5,
				},
				"2a": {
					"3a": 1.25,
					"3b": 1.0,
				},
				"2b": {
					"3a": 1.5,
					"3b": 1.25,
				},
				"2c": {
					"3a": 0.75,
					"3b": 1.0,
				},
				"3a": {
					"goal": 1.0,
				},
				"3b": {
					"goal": 0.75,
				},
			},
			maxExpansions: -1,
			expectedPath:  []string{"start", "1b", "2c", "3a", "goal"},
		},
		"backtracking-first-is-better": {
			inMap: map[string]map[string]float64{
				"start": {
					"left1":  1.0,
					"right1": 3.0,
				},
				"right1": {
					"goal": 1.0,
				},
				"left1": {
					"left2": 1.0,
				},
				"left2": {
					"left3": 1.0,
				},
				"left3": {
					"goal": 0.1,
				},
			},
			maxExpansions: -1,
			expectedPath:  []string{"start", "left1", "left2", "left3", "goal"},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			coster := func(src, dst interface{}) float64 {
				srcS := src.(string)
				dstS := dst.(string)
				subMap, found := tc.inMap[srcS]
				if !found {
					return math.MaxFloat64
				}
				cost, found := subMap[dstS]
				if !found {
					return math.MaxFloat64
				}
				return cost
			}
			isGoal := func(n interface{}) bool {
				ns := n.(string)
				return ns == "goal"
			}
			nGen := func(n interface{}) []interface{} {
				key := n.(string)
				subMap, found := tc.inMap[key]
				if !found {
					return nil
				}
				neighbors := make([]interface{}, 0, len(subMap))
				for neighborKey := range subMap {
					neighbors = append(neighbors, neighborKey)
				}
				return neighbors
			}

			cameFrom, _, final := DijkstraFindPath("start", coster, isGoal, nGen, tc.maxExpansions)

			// Rebuild the path from final to origin
			var path []string
			k, ok := final.(string)
			if ok {
				for {
					path = append(path, k)
					var found bool
					k, found = cameFrom[k].(string)
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
			}

			if !reflect.DeepEqual(path, tc.expectedPath) {
				t.Errorf("expected %v, got %v", tc.expectedPath, path)
			}
		})
	}
}

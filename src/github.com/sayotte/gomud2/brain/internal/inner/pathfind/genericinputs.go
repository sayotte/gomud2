package pathfind

type NodeCoster func(src, dst interface{}) float64
type NodeEstimator func(dst interface{}) float64
type NodeIsGoaler func(n interface{}) bool
type NeighborGenerator func(n interface{}) []interface{}

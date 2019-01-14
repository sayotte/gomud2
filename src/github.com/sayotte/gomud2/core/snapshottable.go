package core

// ALGORITHM FOR SNAPSHOTS
// The network of all items known to a Zone (all snapshottable things) is a
//   DAG; if we visit all items in the set, in any order, and do a DFS down
//   the subtree of dependencies (snapshotDependencies()) ensuing from each
//   item, we're left with a topological ordering.
// Do this DFS; dependencies should be yielded earlier in the list than their
//   dependents, i.e. for each node do:
//     return append([]myDependencies, self)
//   We do it this way because a "snapshotDependents()" is awkward to implement
//   and appending is cheaper than pre-pending.
// This yields an overall ordering from no-deps (front of list) to most-deps
//   (back of list).
// Iterate over the list, calling snapshot() on each item, yielding a list
//   of events that can be replayed in that exact order to reconstruct the
//   current state. These events should have their sequence numbers all set
//   to the same value; namely, the value of the "real" event to which the
//   snapshot corresponds.
// Store the list of events / snapshot.

type snapshottable interface {
	snapshotDependencies() []snapshottable
	snapshot(sequenceNum uint64) Event
}

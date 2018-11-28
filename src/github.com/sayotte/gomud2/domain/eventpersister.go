package domain

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/rpc"
)

type EventRetriever interface {
	RetrieveAllForZone(uuid uuid.UUID) (<-chan rpc.Response, error)
	RetrieveUpToSequenceNumsForZone(endNum uint64, zoneID uuid.UUID) (<-chan rpc.Response, error)
}

type EventPersister interface {
	PersistEvent(e Event) error
}

type SnapshotPersister interface {
	PersistSnapshot(uuid.UUID, uint64, []Event) error
}

type SnapshotHelper interface {
	EventRetriever
	SnapshotPersister
}

type DoEverythingStore interface {
	EventRetriever
	EventPersister
	SnapshotPersister
}

package core

import (
	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/rpc"
)

type EventPersister interface {
	PersistEvent(e Event) error
}

type DataStore interface {
	RetrieveAllEventsForZone(uuid uuid.UUID) (<-chan rpc.Response, error)
	RetrieveEventsUpToSequenceNumForZone(endNum uint64, zoneID uuid.UUID) (<-chan rpc.Response, error)
	EventPersister
	PersistSnapshot(uuid.UUID, uint64, []Event) error
}

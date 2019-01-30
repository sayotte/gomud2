package core

import (
	"github.com/satori/go.uuid"
)

const (
	EventTypeActorMove = iota
	EventTypeActorAdminRelocate
	EventTypeActorAddToZone
	EventTypeActorRemoveFromZone
	EventTypeActorMigrateIn
	EventTypeActorMigrateOut
	EventTypeLocationAddToZone
	EventTypeLocationRemoveFromZone
	EventTypeLocationUpdate
	EventTypeExitAddToZone
	EventTypeExitUpdate
	EventTypeExitRemoveFromZone
	EventTypeObjectAddToZone
	EventTypeObjectRemoveFromZone
	EventTypeObjectMove
	EventTypeObjectAdminRelocate
	EventTypeObjectMigrateIn
	EventTypeObjectMigrateOut
	EventTypeZoneSetDefaultLocation
)

type Event interface {
	Type() int
	Version() int
	AggregateId() uuid.UUID
	SequenceNumber() uint64
	SetSequenceNumber(num uint64)
	ShouldPersist() bool
}

type eventGeneric struct {
	eventType      int
	version        int
	aggregateId    uuid.UUID
	sequenceNumber uint64
	shouldPersist  bool
}

func (eg eventGeneric) Type() int {
	return eg.eventType
}

func (eg eventGeneric) Version() int {
	return eg.version
}

func (eg eventGeneric) AggregateId() uuid.UUID {
	return eg.aggregateId
}

func (eg eventGeneric) SequenceNumber() uint64 {
	return eg.sequenceNumber
}

func (eg *eventGeneric) SetSequenceNumber(num uint64) {
	eg.sequenceNumber = num
}

func (eg eventGeneric) ShouldPersist() bool {
	return eg.shouldPersist
}

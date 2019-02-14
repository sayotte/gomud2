package core

import (
	"github.com/satori/go.uuid"
	"time"
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
	EventTypeCombatMeleeDamage
	EventTypeCombatDodge
)

type Event interface {
	Type() int
	Timestamp() time.Time
	SetTimestamp(t time.Time)
	Version() int
	AggregateId() uuid.UUID
	SequenceNumber() uint64
	SetSequenceNumber(num uint64)
	ShouldPersist() bool
}

type eventGeneric struct {
	SequenceNum       uint64
	TimeStamp         time.Time
	EventTypeNum      int
	VersionNum        int
	AggregateID       uuid.UUID
	ShouldPersistBool bool
}

func (eg eventGeneric) Type() int {
	return eg.EventTypeNum
}

func (eg eventGeneric) Timestamp() time.Time {
	return eg.TimeStamp
}

func (eg *eventGeneric) SetTimestamp(t time.Time) {
	eg.TimeStamp = t
}

func (eg eventGeneric) Version() int {
	return eg.VersionNum
}

func (eg eventGeneric) AggregateId() uuid.UUID {
	return eg.AggregateID
}

func (eg eventGeneric) SequenceNumber() uint64 {
	return eg.SequenceNum
}

func (eg *eventGeneric) SetSequenceNumber(num uint64) {
	eg.SequenceNum = num
}

func (eg eventGeneric) ShouldPersist() bool {
	return eg.ShouldPersistBool
}

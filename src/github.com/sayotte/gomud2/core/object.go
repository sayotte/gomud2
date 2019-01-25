package core

import (
	"errors"
	"fmt"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/rpc"
	myuuid "github.com/sayotte/gomud2/uuid"
)

func NewObject(id uuid.UUID, name string, loc *Location, zone *Zone) *Object {
	newID := id
	if uuid.Equal(id, uuid.Nil) {
		newID = myuuid.NewId()
	}
	return &Object{
		id:          newID,
		name:        name,
		location:    loc,
		zone:        zone,
		requestChan: make(chan rpc.Request),
	}
}

type Object struct {
	id       uuid.UUID
	name     string
	location *Location
	zone     *Zone
	// This is the channel where the Zone picks up new events related to this
	// Object. This should never be directly exposed by an accessor; public methods
	// should create events and send them to the channel.
	requestChan chan rpc.Request
}

func (o Object) ID() uuid.UUID {
	return o.id
}

func (o Object) Name() string {
	return o.name
}

func (o Object) Location() *Location {
	return o.location
}

func (o *Object) setLocation(loc *Location) {
	o.location = loc
}

func (o *Object) setZone(zone *Zone) {
	o.zone = zone
}

func (o *Object) Move(from, to *Location) error {
	fmt.Printf("object %q, moving from %q to %q\n", o.name, from.ShortDescription(), to.ShortDescription())
	if from.Zone() != to.Zone() {
		return fmt.Errorf("cross-zone moves not yet supported")
	}
	e := NewObjectMoveEvent(
		from.ID(),
		to.ID(),
		o.id,
		o.zone.ID(),
	)
	_, err := o.syncRequestToZone(e)
	return err
}

func (o *Object) AdminRelocate(to *Location) error {
	fromLoc := o.location
	if fromLoc != nil && fromLoc.Zone() != to.Zone() {
		return errors.New("cannot AdminRelocate across Zones")
	}
	e := NewObjectAdminRelocateEvent(o.id, to.ID(), to.Zone().ID())
	_, err := o.syncRequestToZone(e)
	return err
}

func (o *Object) syncRequestToZone(e Event) (interface{}, error) {
	req := rpc.NewRequest(e)
	o.requestChan <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (o Object) snapshot(sequenceNum uint64) Event {
	e := NewObjectAddToZoneEvent(
		o.name,
		o.id,
		o.location.ID(),
		o.zone.ID(),
	)
	e.SetSequenceNumber(sequenceNum)
	return e
}

func (o Object) snapshotDependencies() []snapshottable {
	return []snapshottable{o.location}
}

type ObjectList []*Object

func (ol ObjectList) IndexOf(obj *Object) (int, error) {
	for i := 0; i < len(ol); i++ {
		if ol[i] == obj {
			return i, nil
		}
	}
	return -1, fmt.Errorf("Object %q not found in list", obj.id)
}

func (ol ObjectList) Copy() ObjectList {
	out := make(ObjectList, len(ol))
	copy(out, ol)
	return out
}

func newObjectAddToZoneCommand(wrapped ObjectAddToZoneEvent) objectAddToZoneCommand {
	return objectAddToZoneCommand{
		commandGeneric{commandType: CommandTypeObjectAddToZone},
		wrapped,
	}
}

type objectAddToZoneCommand struct {
	commandGeneric
	wrappedEvent ObjectAddToZoneEvent
}

func NewObjectAddToZoneEvent(name string, objectId, startingLocationId, zoneId uuid.UUID) ObjectAddToZoneEvent {
	return ObjectAddToZoneEvent{
		&eventGeneric{
			eventType:     EventTypeObjectAddToZone,
			version:       1,
			aggregateId:   zoneId,
			shouldPersist: true,
		},
		objectId,
		name,
		startingLocationId,
	}
}

type ObjectAddToZoneEvent struct {
	*eventGeneric
	objectId           uuid.UUID
	name               string
	startingLocationId uuid.UUID
}

func (oatze ObjectAddToZoneEvent) ObjectID() uuid.UUID {
	return oatze.objectId
}

func (oatze ObjectAddToZoneEvent) Name() string {
	return oatze.name
}

func (oatze ObjectAddToZoneEvent) StartingLocationID() uuid.UUID {
	return oatze.startingLocationId
}

func newObjectRemoveFromZoneCommand(wrapped ObjectRemoveFromZoneEvent) objectRemoveFromZoneCommand {
	return objectRemoveFromZoneCommand{
		commandGeneric{commandType: CommandTypeObjectRemoveFromZone},
		wrapped,
	}
}

type objectRemoveFromZoneCommand struct {
	commandGeneric
	wrappedEvent ObjectRemoveFromZoneEvent
}

func NewObjectRemoveFromZoneEvent(name string, objectID, zoneID uuid.UUID) ObjectRemoveFromZoneEvent {
	return ObjectRemoveFromZoneEvent{
		&eventGeneric{
			eventType:     EventTypeObjectRemoveFromZone,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		objectID,
		name,
	}
}

type ObjectRemoveFromZoneEvent struct {
	*eventGeneric
	objectID uuid.UUID
	name     string
}

func (orfze ObjectRemoveFromZoneEvent) ObjectID() uuid.UUID {
	return orfze.objectID
}

func (orfze ObjectRemoveFromZoneEvent) Name() string {
	return orfze.name
}

func newObjectMoveCommand(wrapped ObjectMoveEvent) objectMoveCommand {
	return objectMoveCommand{
		commandGeneric{commandType: CommandTypeObjectMove},
		wrapped,
	}
}

type objectMoveCommand struct {
	commandGeneric
	wrappedEvent ObjectMoveEvent
}

func NewObjectMoveEvent(fromLocationId, toLocationId, objectId, zoneId uuid.UUID) ObjectMoveEvent {
	return ObjectMoveEvent{
		&eventGeneric{
			eventType:     EventTypeObjectMove,
			version:       1,
			aggregateId:   zoneId,
			shouldPersist: true,
		},
		fromLocationId,
		toLocationId,
		objectId,
	}
}

type ObjectMoveEvent struct {
	*eventGeneric
	fromLocationId uuid.UUID
	toLocationId   uuid.UUID
	objectId       uuid.UUID
}

func (ome ObjectMoveEvent) FromLocationID() uuid.UUID {
	return ome.fromLocationId
}

func (ome ObjectMoveEvent) ToLocationID() uuid.UUID {
	return ome.toLocationId
}

func (ome ObjectMoveEvent) ObjectID() uuid.UUID {
	return ome.objectId
}

func newObjectAdminRelocateCommand(wrapped ObjectAdminRelocateEvent) objectAdminRelocateCommand {
	return objectAdminRelocateCommand{
		commandGeneric{commandType: CommandTypeObjectAdminRelocate},
		wrapped,
	}
}

type objectAdminRelocateCommand struct {
	commandGeneric
	wrappedEvent ObjectAdminRelocateEvent
}

func NewObjectAdminRelocateEvent(objectID, locID, zoneID uuid.UUID) ObjectAdminRelocateEvent {
	return ObjectAdminRelocateEvent{
		eventGeneric: &eventGeneric{
			eventType:     EventTypeObjectAdminRelocate,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		ObjectID:     objectID,
		ToLocationID: locID,
	}
}

type ObjectAdminRelocateEvent struct {
	*eventGeneric
	ObjectID     uuid.UUID
	ToLocationID uuid.UUID
}

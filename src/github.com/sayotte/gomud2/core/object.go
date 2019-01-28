package core

import (
	"fmt"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/rpc"
	myuuid "github.com/sayotte/gomud2/uuid"
)

func NewObject(id uuid.UUID, name string, container Container, zone *Zone) *Object {
	newID := id
	if uuid.Equal(id, uuid.Nil) {
		newID = myuuid.NewId()
	}
	return &Object{
		id:          newID,
		name:        name,
		container:   container,
		zone:        zone,
		requestChan: make(chan rpc.Request),
	}
}

type Object struct {
	id        uuid.UUID
	name      string
	container Container
	zone      *Zone

	containerCapacity int
	containedObjects  ObjectList
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

func (o Object) Container() Container {
	return o.container
}

func (o *Object) setContainer(c Container) {
	o.container = c
}

func (o *Object) Zone() *Zone {
	return o.zone
}

func (o *Object) setZone(zone *Zone) {
	o.zone = zone
}

func (o *Object) Objects() ObjectList {
	return o.containedObjects.Copy()
}

func (o *Object) addObject(object *Object) error {
	_, err := o.containedObjects.IndexOf(object)
	if err == nil {
		return fmt.Errorf("Object %q already present in Object/Container %q", object.ID(), o.id)
	}
	o.containedObjects = append(o.containedObjects, object)
	return nil
}

func (o *Object) removeObject(object *Object) {
	o.containedObjects = o.containedObjects.Remove(object)
}

func (o *Object) Capacity() int {
	return o.containerCapacity
}

func (o *Object) ContainsObject(object *Object) bool {
	_, err := o.containedObjects.IndexOf(object)
	return err == nil
}

func (o *Object) Observers() ObserverList {
	return o.container.Observers()
}

func (o *Object) Move(from, to Container) error {
	cmd := newObjectMoveCommand(o, from, to)
	_, err := o.syncRequestToZone(cmd)
	return err
}

func (o *Object) AdminRelocate(to Container) error {
	e := NewObjectAdminRelocateEvent(o.id, o.Zone().ID())
	switch to.(type) {
	case *Location:
		e.ToLocationContainerID = o.container.ID()
	case *Actor:
		e.ToActorContainerID = o.container.ID()
	case *Object:
		e.ToObjectContainerID = o.container.ID()
	}

	cmd := newObjectAdminRelocateCommand(e)
	_, err := o.syncRequestToZone(cmd)
	return err
}

func (o *Object) syncRequestToZone(c Command) (interface{}, error) {
	req := rpc.NewRequest(c)
	o.requestChan <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (o Object) snapshot(sequenceNum uint64) Event {
	e := NewObjectAddToZoneEvent(
		o.name,
		o.id,
		uuid.Nil,
		uuid.Nil,
		uuid.Nil,
		o.zone.ID(),
	)
	switch o.container.(type) {
	case *Location:
		e.LocationContainerID = o.container.ID()
	case *Actor:
		e.ActorContainerID = o.container.ID()
	case *Object:
		e.ObjectContainerID = o.container.ID()
	}
	e.SetSequenceNumber(sequenceNum)
	return e
}

func (o Object) snapshotDependencies() []snapshottable {
	return []snapshottable{o.container.(snapshottable)}
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

func (ol ObjectList) Remove(object *Object) ObjectList {
	idx, err := ol.IndexOf(object)
	if err != nil {
		return ol
	}
	return append(ol[:idx], ol[idx+1:]...)
}

func newObjectAddToZoneCommand(wrapped *ObjectAddToZoneEvent) objectAddToZoneCommand {
	return objectAddToZoneCommand{
		commandGeneric{commandType: CommandTypeObjectAddToZone},
		wrapped,
	}
}

type objectAddToZoneCommand struct {
	commandGeneric
	wrappedEvent *ObjectAddToZoneEvent
}

func NewObjectAddToZoneEvent(name string, objectId, locationContainerID, actorContainerID, objectContainerID, zoneId uuid.UUID) *ObjectAddToZoneEvent {
	return &ObjectAddToZoneEvent{
		eventGeneric: &eventGeneric{
			eventType:     EventTypeObjectAddToZone,
			version:       1,
			aggregateId:   zoneId,
			shouldPersist: true,
		},
		ObjectID:            objectId,
		Name:                name,
		LocationContainerID: locationContainerID,
		ActorContainerID:    actorContainerID,
		ObjectContainerID:   objectContainerID,
	}
}

type ObjectAddToZoneEvent struct {
	*eventGeneric
	ObjectID                                                 uuid.UUID
	Name                                                     string
	LocationContainerID, ActorContainerID, ObjectContainerID uuid.UUID
}

func newObjectRemoveFromZoneCommand(wrapped *ObjectRemoveFromZoneEvent) objectRemoveFromZoneCommand {
	return objectRemoveFromZoneCommand{
		commandGeneric{commandType: CommandTypeObjectRemoveFromZone},
		wrapped,
	}
}

type objectRemoveFromZoneCommand struct {
	commandGeneric
	wrappedEvent *ObjectRemoveFromZoneEvent
}

func NewObjectRemoveFromZoneEvent(name string, objectID, zoneID uuid.UUID) *ObjectRemoveFromZoneEvent {
	return &ObjectRemoveFromZoneEvent{
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
	ObjectID uuid.UUID
	Name     string
}

func newObjectMoveCommand(obj *Object, fromContainer, toContainer Container) objectMoveCommand {
	return objectMoveCommand{
		commandGeneric: commandGeneric{commandType: CommandTypeObjectMove},
		obj:            obj,
		fromContainer:  fromContainer,
		toContainer:    toContainer,
	}
}

type objectMoveCommand struct {
	commandGeneric
	obj           *Object
	fromContainer Container
	toContainer   Container
}

func NewObjectMoveEvent(objID, zoneID uuid.UUID) *ObjectMoveEvent {
	return &ObjectMoveEvent{
		eventGeneric: &eventGeneric{
			eventType:     EventTypeObjectMove,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		ObjectID: objID,
	}
}

type ObjectMoveEvent struct {
	*eventGeneric
	ObjectID                                                             uuid.UUID
	FromLocationContainerID, FromActorContainerID, FromObjectContainerID uuid.UUID
	ToLocationContainerID, ToActorContainerID, ToObjectContainerID       uuid.UUID
}

func newObjectAdminRelocateCommand(wrapped *ObjectAdminRelocateEvent) objectAdminRelocateCommand {
	return objectAdminRelocateCommand{
		commandGeneric{commandType: CommandTypeObjectAdminRelocate},
		wrapped,
	}
}

type objectAdminRelocateCommand struct {
	commandGeneric
	wrappedEvent *ObjectAdminRelocateEvent
}

func NewObjectAdminRelocateEvent(objectID, zoneID uuid.UUID) *ObjectAdminRelocateEvent {
	return &ObjectAdminRelocateEvent{
		eventGeneric: &eventGeneric{
			eventType:     EventTypeObjectAdminRelocate,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		ObjectID: objectID,
	}
}

type ObjectAdminRelocateEvent struct {
	*eventGeneric
	ObjectID                                                       uuid.UUID
	ToLocationContainerID, ToActorContainerID, ToObjectContainerID uuid.UUID
}

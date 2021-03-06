package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/rpc"
	myuuid "github.com/sayotte/gomud2/uuid"
)

func NewObject(id uuid.UUID, name, desc string, keywords []string, container Container, capacity int, zone *Zone, attrs ObjectAttributes) *Object {
	newID := id
	if uuid.Equal(id, uuid.Nil) {
		newID = myuuid.NewId()
	}
	return &Object{
		id:                newID,
		name:              name,
		description:       desc,
		keywords:          keywords,
		container:         container,
		containerCapacity: capacity,
		zone:              zone,
		attributes:        attrs,
	}
}

type Object struct {
	id             uuid.UUID
	name           string
	description    string
	inventorySlots int // FIXME this is a dupe of containerCapacity, and neither is currently used in all places
	keywords       []string
	container      Container
	zone           *Zone

	containerCapacity int
	containedObjects  ObjectList

	attributes ObjectAttributes
}

func (o Object) ID() uuid.UUID {
	return o.id
}

func (o Object) Name() string {
	return o.name
}

func (o Object) Description() string {
	return o.description
}

func (o Object) InventorySlots() int {
	return o.inventorySlots
}

func (o Object) Keywords() []string {
	out := make([]string, len(o.keywords))
	copy(out, o.keywords)
	return out
}

func (o Object) Container() Container {
	return o.container
}

func (o *Object) setContainer(c Container) {
	o.container = c
}

func (o *Object) SubcontainerFor(obj *Object) string {
	return ContainerDefaultSubcontainer
}

func (o *Object) Location() *Location {
	return o.container.Location()
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

func (o *Object) addObject(object *Object, subcontainer string) error {
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

func (o *Object) checkMoveObjectToSubcontainer(object *Object, oldSub, newSub string) error {
	return errors.New("Object does not implement subcontainers")
}

func (o *Object) moveObjectToSubcontainer(object *Object, oldSub, newSub string) error {
	if newSub == ContainerDefaultSubcontainer {
		return nil
	}
	return errors.New("Object does not implement subcontainers")
}

func (o *Object) Capacity() int {
	return o.containerCapacity
}

func (o *Object) ContainsObject(object *Object) bool {
	_, err := o.containedObjects.IndexOf(object)
	return err == nil
}

func (o *Object) Attributes() ObjectAttributes {
	return o.attributes
}

func (o *Object) Observers() ObserverList {
	return o.container.Observers()
}

func (o *Object) Move(from, to Container, who *Actor, toSubcontainer string) error {
	cmd := newObjectMoveCommand(o, who, from, to, toSubcontainer)
	_, err := o.syncRequestToZone(cmd)
	return err
}

func (o *Object) MoveToSubcontainer(subcontainer string, actor *Actor) error {
	if !actor.ContainsObject(o) {
		return errors.New("Actor doesn't possess that Object")
	}

	fromSub := actor.SubcontainerFor(o)
	cmd := newObjectMoveSubcontainerCommand(o, fromSub, subcontainer, actor)
	_, err := o.syncRequestToZone(cmd)
	return err
}

func (o *Object) AdminRelocate(to Container, toSubcontainer string) error {
	e := NewObjectAdminRelocateEvent(o.id, o.Zone().ID())
	switch to.(type) {
	case *Location:
		e.ToLocationContainerID = o.container.ID()
	case *Actor:
		e.ToActorContainerID = o.container.ID()
	case *Object:
		e.ToObjectContainerID = o.container.ID()
	}
	e.ToSubcontainer = toSubcontainer

	cmd := newObjectAdminRelocateCommand(e)
	_, err := o.syncRequestToZone(cmd)
	return err
}

func (o *Object) syncRequestToZone(c Command) (interface{}, error) {
	req := rpc.NewRequest(c)
	o.zone.requestChan() <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (o *Object) snapshot(sequenceNum uint64) Event {
	e := NewObjectAddToZoneEvent(
		o.name,
		o.description,
		o.keywords,
		o.containerCapacity,
		o.id,
		uuid.Nil,
		uuid.Nil,
		uuid.Nil,
		o.zone.ID(),
		o.container.SubcontainerFor(o),
		o.attributes,
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

func NewObjectAddToZoneEvent(name, desc string, keywords []string, capacity int, objectId, locationContainerID, actorContainerID, objectContainerID, zoneId uuid.UUID, subcontainer string, attrs ObjectAttributes) *ObjectAddToZoneEvent {
	return &ObjectAddToZoneEvent{
		eventGeneric: &eventGeneric{
			EventTypeNum:      EventTypeObjectAddToZone,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneId,
			ShouldPersistBool: true,
		},
		ObjectID:            objectId,
		Name:                name,
		Description:         desc,
		Keywords:            keywords,
		LocationContainerID: locationContainerID,
		ActorContainerID:    actorContainerID,
		ObjectContainerID:   objectContainerID,
		Subcontainer:        subcontainer,
		Capacity:            capacity,
		Attributes:          attrs,
	}
}

type ObjectAddToZoneEvent struct {
	*eventGeneric
	ObjectID                                                 uuid.UUID
	Name, Description                                        string
	Keywords                                                 []string
	LocationContainerID, ActorContainerID, ObjectContainerID uuid.UUID
	Subcontainer                                             string
	Capacity                                                 int
	Attributes                                               ObjectAttributes
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
			EventTypeNum:      EventTypeObjectRemoveFromZone,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
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

func newObjectMoveCommand(obj *Object, actor *Actor, fromContainer, toContainer Container, toSubcontainer string) objectMoveCommand {
	return objectMoveCommand{
		commandGeneric: commandGeneric{commandType: CommandTypeObjectMove},
		obj:            obj,
		actor:          actor,
		fromContainer:  fromContainer,
		toContainer:    toContainer,
		toSubcontainer: toSubcontainer,
	}
}

type objectMoveCommand struct {
	commandGeneric
	obj            *Object
	actor          *Actor
	fromContainer  Container
	toContainer    Container
	toSubcontainer string
}

func NewObjectMoveEvent(objID, actorID, zoneID uuid.UUID) *ObjectMoveEvent {
	return &ObjectMoveEvent{
		eventGeneric: &eventGeneric{
			EventTypeNum:      EventTypeObjectMove,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
		},
		ObjectID: objID,
		ActorID:  actorID,
	}
}

type ObjectMoveEvent struct {
	*eventGeneric
	ObjectID                                                             uuid.UUID
	ActorID                                                              uuid.UUID
	FromLocationContainerID, FromActorContainerID, FromObjectContainerID uuid.UUID
	ToLocationContainerID, ToActorContainerID, ToObjectContainerID       uuid.UUID
	ToSubcontainer                                                       string
}

func newObjectMoveSubcontainerCommand(obj *Object, fromSub, toSub string, actor *Actor) objectMoveSubcontainerCommand {
	return objectMoveSubcontainerCommand{
		commandGeneric:   commandGeneric{commandType: CommandTypeObjectMoveSubcontainer},
		obj:              obj,
		fromSubcontainer: fromSub,
		toSubcontainer:   toSub,
		actor:            actor,
	}
}

type objectMoveSubcontainerCommand struct {
	commandGeneric
	obj              *Object
	fromSubcontainer string
	toSubcontainer   string
	actor            *Actor
}

func NewObjectMoveSubcontainerEvent(objId, actorID, zoneID uuid.UUID, fromSub, toSub string) *ObjectMoveSubcontainerEvent {
	return &ObjectMoveSubcontainerEvent{
		eventGeneric: &eventGeneric{
			EventTypeNum:      EventTypeObjectMoveSubcontainer,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
		},
		ObjectID:         objId,
		FromSubcontainer: fromSub,
		ToSubcontainer:   toSub,
		ActorID:          actorID,
	}
}

type ObjectMoveSubcontainerEvent struct {
	*eventGeneric
	ObjectID                         uuid.UUID
	FromSubcontainer, ToSubcontainer string
	ActorID                          uuid.UUID
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
			EventTypeNum:      EventTypeObjectAdminRelocate,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
		},
		ObjectID: objectID,
	}
}

type ObjectAdminRelocateEvent struct {
	*eventGeneric
	ObjectID                                                       uuid.UUID
	ToLocationContainerID, ToActorContainerID, ToObjectContainerID uuid.UUID
	ToSubcontainer                                                 string
}

func NewObjectMigrateInEvent(name, desc string, keywords []string, capacity int, objID, fromZoneID, locContainerID, actorContainerID, objContainerID, zoneID uuid.UUID, subcontainer string, attrs ObjectAttributes) *ObjectMigrateInEvent {
	return &ObjectMigrateInEvent{
		eventGeneric: &eventGeneric{
			EventTypeNum:      EventTypeObjectMigrateIn,
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
		},
		ObjectID:            objID,
		Name:                name,
		Description:         desc,
		Keywords:            keywords,
		FromZoneID:          fromZoneID,
		LocationContainerID: locContainerID,
		ActorContainerID:    actorContainerID,
		ObjectContainerID:   objContainerID,
		Capacity:            capacity,
		Subcontainer:        subcontainer,
		Attributes:          attrs,
	}
}

type ObjectMigrateInEvent struct {
	*eventGeneric
	ObjectID                                                 uuid.UUID
	Name, Description                                        string
	Keywords                                                 []string
	FromZoneID                                               uuid.UUID
	LocationContainerID, ActorContainerID, ObjectContainerID uuid.UUID
	Subcontainer                                             string
	Capacity                                                 int
	Attributes                                               ObjectAttributes
}

func NewObjectMigrateOutEvent(name string, objID, toZoneID, zoneID uuid.UUID) *ObjectMigrateOutEvent {
	return &ObjectMigrateOutEvent{
		eventGeneric: &eventGeneric{
			EventTypeNum:      EventTypeObjectMigrateOut,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
		},
		ObjectID: objID,
		Name:     name,
		ToZoneID: toZoneID,
	}
}

type ObjectMigrateOutEvent struct {
	*eventGeneric
	ObjectID uuid.UUID
	Name     string
	ToZoneID uuid.UUID
}

package core

import (
	"errors"
	"fmt"
	"sync"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/rpc"
	myuuid "github.com/sayotte/gomud2/uuid"
)

const actorInventoryCapacity = 15

func NewActor(id uuid.UUID, name string, location *Location, zone *Zone) *Actor {
	newID := id
	if uuid.Equal(id, uuid.Nil) {
		newID = myuuid.NewId()
	}
	return &Actor{
		id:          newID,
		name:        name,
		location:    location,
		zone:        zone,
		rwlock:      &sync.RWMutex{},
		requestChan: make(chan rpc.Request),
	}
}

type Actor struct {
	id        uuid.UUID
	name      string
	location  *Location
	zone      *Zone
	observers ObserverList

	inventoryObjects ObjectList

	rwlock *sync.RWMutex
	// This is the channel where the Zone picks up new events related to this
	// actor. This should never be directly exposed by an accessor; public methods
	// should create events and send them to the channel.
	requestChan chan rpc.Request
}

func (a *Actor) ID() uuid.UUID {
	return a.id
}

func (a *Actor) Observers() ObserverList {
	a.rwlock.RLock()
	defer a.rwlock.RUnlock()
	return a.observers.Copy()
}

func (a *Actor) AddObserver(o Observer) {
	a.rwlock.Lock()
	defer a.rwlock.Unlock()
	a.observers = append(a.observers, o)
}

func (a *Actor) RemoveObserver(o Observer) {
	a.rwlock.Lock()
	defer a.rwlock.Unlock()
	a.observers = a.observers.Remove(o)
}

func (a *Actor) Name() string {
	return a.name
}

func (a *Actor) Location() *Location {
	a.rwlock.RLock()
	defer a.rwlock.RUnlock()
	return a.location
}

func (a *Actor) setLocation(loc *Location) {
	a.location = loc
}

func (a *Actor) Capacity() int {
	return actorInventoryCapacity
}

func (a *Actor) Objects() ObjectList {
	return a.inventoryObjects.Copy()
}

func (a *Actor) ContainsObject(o *Object) bool {
	_, err := a.inventoryObjects.IndexOf(o)
	return err == nil
}

func (a *Actor) addObject(o *Object) error {
	_, err := a.inventoryObjects.IndexOf(o)
	if err == nil {
		return fmt.Errorf("Object %q already present in inventory for Actor %q", o.ID(), a.id)
	}
	a.inventoryObjects = append(a.inventoryObjects, o)
	return nil
}

func (a *Actor) removeObject(o *Object) {
	a.inventoryObjects = a.inventoryObjects.Remove(o)
}

func (a *Actor) Move(from, to *Location) error {
	if from.Zone() != to.Zone() {
		return fmt.Errorf("cross-zone moves should use the World.MigrateZone() API call")
	}

	e := NewActorMoveEvent(
		from.ID(),
		to.ID(),
		a.id,
		a.zone.ID(),
	)
	cmd := newActorMoveCommand(e)
	_, err := a.syncRequestToZone(cmd)

	return err
}

func (a *Actor) AdminRelocate(to *Location) error {
	fromLoc := a.location
	if fromLoc != nil && fromLoc.Zone() != to.Zone() {
		return errors.New("cannot AdminRelocate across Zones")
	}
	e := NewActorAdminRelocateEvent(a.id, to.ID(), to.Zone().ID())
	c := newActorAdminRelocateCommand(e)
	_, err := a.syncRequestToZone(c)
	return err
}

func (a *Actor) Zone() *Zone {
	a.rwlock.RLock()
	defer a.rwlock.RUnlock()
	return a.zone
}

func (a *Actor) setZone(z *Zone) {
	a.rwlock.Lock()
	defer a.rwlock.Unlock()
	a.zone = z
}

func (a *Actor) syncRequestToZone(c Command) (interface{}, error) {
	req := rpc.NewRequest(c)
	a.requestChan <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (a Actor) snapshot(sequenceNum uint64) Event {
	e := NewActorAddToZoneEvent(
		a.Name(),
		a.id,
		a.location.ID(),
		a.zone.ID(),
	)
	e.SetSequenceNumber(sequenceNum)
	return e
}

func (a Actor) snapshotDependencies() []snapshottable {
	return []snapshottable{a.location}
}

type ActorList []*Actor

func (al ActorList) IndexOf(actor *Actor) (int, error) {
	for i := 0; i < len(al); i++ {
		if al[i] == actor {
			return i, nil
		}
	}
	return -1, fmt.Errorf("Actor %q not found in list", actor.id)
}

func (al ActorList) Copy() ActorList {
	out := make(ActorList, len(al))
	copy(out, al)
	return out
}

func (al ActorList) Remove(actor *Actor) ActorList {
	idx, err := al.IndexOf(actor)
	if err != nil {
		return al
	}
	return append(al[:idx], al[idx+1:]...)
}

func newActorMoveCommand(wrapped *ActorMoveEvent) actorMoveCommand {
	return actorMoveCommand{
		commandGeneric{commandType: CommandTypeActorMove},
		wrapped,
	}
}

type actorMoveCommand struct {
	commandGeneric
	wrappedEvent *ActorMoveEvent
}

func NewActorMoveEvent(fromLocationId, toLocationId, actorId, zoneId uuid.UUID) *ActorMoveEvent {
	return &ActorMoveEvent{
		&eventGeneric{
			eventType:     EventTypeActorMove,
			version:       1,
			aggregateId:   zoneId,
			shouldPersist: true,
		},
		fromLocationId,
		toLocationId,
		actorId,
	}
}

type ActorMoveEvent struct {
	*eventGeneric
	fromLocationId uuid.UUID
	toLocationId   uuid.UUID
	actorId        uuid.UUID
}

func (ame ActorMoveEvent) FromToActorIDs() (uuid.UUID, uuid.UUID, uuid.UUID) {
	return ame.fromLocationId, ame.toLocationId, ame.actorId
}

func newActorAdminRelocateCommand(wrapped *ActorAdminRelocateEvent) actorAdminRelocateCommand {
	return actorAdminRelocateCommand{
		commandGeneric{commandType: CommandTypeActorAdminRelocate},
		wrapped,
	}
}

type actorAdminRelocateCommand struct {
	commandGeneric
	wrappedEvent *ActorAdminRelocateEvent
}

func NewActorAdminRelocateEvent(actorID, locID, zoneID uuid.UUID) *ActorAdminRelocateEvent {
	return &ActorAdminRelocateEvent{
		eventGeneric: &eventGeneric{
			eventType:     EventTypeActorAdminRelocate,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		ActorID:      actorID,
		ToLocationID: locID,
	}
}

type ActorAdminRelocateEvent struct {
	*eventGeneric
	ActorID      uuid.UUID
	ToLocationID uuid.UUID
}

func newActorAddToZoneCommand(wrapped *ActorAddToZoneEvent) actorAddToZoneCommand {
	return actorAddToZoneCommand{
		commandGeneric{commandType: CommandTypeActorAddToZone},
		wrapped,
	}
}

type actorAddToZoneCommand struct {
	commandGeneric
	wrappedEvent *ActorAddToZoneEvent
}

func NewActorAddToZoneEvent(name string, actorId, startingLocationId, zoneId uuid.UUID) *ActorAddToZoneEvent {
	return &ActorAddToZoneEvent{
		&eventGeneric{
			eventType:     EventTypeActorAddToZone,
			version:       1,
			aggregateId:   zoneId,
			shouldPersist: true,
		},
		actorId,
		name,
		startingLocationId,
	}
}

type ActorAddToZoneEvent struct {
	*eventGeneric
	actorId            uuid.UUID
	name               string
	startingLocationId uuid.UUID
}

func (aatze ActorAddToZoneEvent) ActorID() uuid.UUID {
	return aatze.actorId
}

func (aatze ActorAddToZoneEvent) Name() string {
	return aatze.name
}

func (aatze ActorAddToZoneEvent) StartingLocationID() uuid.UUID {
	return aatze.startingLocationId
}

func newActorRemoveFromZoneCommand(wrapped *ActorRemoveFromZoneEvent) *actorRemoveFromZoneCommand {
	return &actorRemoveFromZoneCommand{
		commandGeneric{commandType: CommandTypeActorRemoveFromZone},
		wrapped,
	}
}

type actorRemoveFromZoneCommand struct {
	commandGeneric
	wrappedEvent *ActorRemoveFromZoneEvent
}

func NewActorRemoveFromZoneEvent(actorID, zoneID uuid.UUID) ActorRemoveFromZoneEvent {
	return ActorRemoveFromZoneEvent{
		&eventGeneric{
			eventType:     EventTypeActorRemoveFromZone,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		actorID,
	}
}

type ActorRemoveFromZoneEvent struct {
	*eventGeneric
	actorID uuid.UUID
}

func (arfze ActorRemoveFromZoneEvent) ActorID() uuid.UUID {
	return arfze.actorID
}

func newActorMigrateInCommand(actor *Actor, from, to *Location, oList ObserverList) *actorMigrateInCommand {
	return &actorMigrateInCommand{
		commandGeneric{commandType: CommandTypeActorMigrateIn},
		actor,
		from,
		to,
		oList,
	}
}

type actorMigrateInCommand struct {
	commandGeneric
	actor     *Actor
	from, to  *Location
	observers ObserverList
}

func NewActorMigrateInEvent(name string, actorID, fromLocID, fromZoneID, toLocID, zoneID uuid.UUID) *ActorMigrateInEvent {
	return &ActorMigrateInEvent{
		eventGeneric: &eventGeneric{
			eventType:     EventTypeActorMigrateIn,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		Name:       name,
		ActorID:    actorID,
		FromLocID:  fromLocID,
		FromZoneID: fromZoneID,
		ToLocID:    toLocID,
	}
}

type ActorMigrateInEvent struct {
	*eventGeneric
	ActorID               uuid.UUID
	Name                  string
	FromLocID, FromZoneID uuid.UUID
	ToLocID               uuid.UUID
}

func newActorMigrateOutCommand(actor *Actor, from, to *Location) *actorMigrateOutCommand {
	return &actorMigrateOutCommand{
		commandGeneric{commandType: CommandTypeActorMigrateOut},
		actor,
		from,
		to,
	}
}

type actorMigrateOutCommand struct {
	commandGeneric
	actor    *Actor
	from, to *Location
}

func NewActorMigrateOutEvent(actorID, fromLocID, toLocID, toZoneID, zoneID uuid.UUID) *ActorMigrateOutEvent {
	return &ActorMigrateOutEvent{
		eventGeneric: &eventGeneric{
			eventType:     EventTypeActorMigrateOut,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		ActorID:   actorID,
		FromLocID: fromLocID,
		ToLocID:   toLocID,
		ToZoneID:  toZoneID,
	}
}

type ActorMigrateOutEvent struct {
	*eventGeneric
	ActorID           uuid.UUID
	FromLocID         uuid.UUID
	ToLocID, ToZoneID uuid.UUID
}

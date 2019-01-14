package core

import (
	"encoding/json"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/rpc"
	myuuid "github.com/sayotte/gomud2/uuid"
	"sync"
)

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
	rwlock    *sync.RWMutex
	// This is the channel where the Zone picks up new events related to this
	// actor. This should never be directly exposed by an accessor; public methods
	// should create events and send them to the channel.
	requestChan chan rpc.Request
}

func (a *Actor) ID() uuid.UUID {
	return a.id
}

func (a *Actor) Observers() []Observer {
	a.rwlock.RLock()
	defer a.rwlock.RUnlock()
	return a.observers
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

func (a *Actor) Move(from, to *Location) error {
	fmt.Printf("actor %q, moving from %q to %q\n", a.Name(), from.ShortDescription, to.ShortDescription)
	if from.Zone != to.Zone {
		return fmt.Errorf("cross-zone moves should use the World.MigrateZone() API call")
	}
	exitExists := false
	for _, edge := range from.OutEdges {
		if edge.Destination == to {
			exitExists = true
			break
		}
	}
	if !exitExists {
		return fmt.Errorf("no exit to that destination from location %q", from.Id)
	}
	e := NewActorMoveEvent(
		from.Id,
		to.Id,
		a.id,
		a.zone.Id,
	)
	_, err := a.syncRequestToZone(e)

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

func (a *Actor) syncRequestToZone(e Event) (interface{}, error) {
	req := rpc.NewRequest(e)
	a.requestChan <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (a Actor) snapshot(sequenceNum uint64) Event {
	e := NewActorAddToZoneEvent(
		a.Name(),
		a.id,
		a.location.Id,
		a.zone.Id,
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

func (ame ActorMoveEvent) MarshalJSON() ([]byte, error) {
	type marshalable ActorMoveEvent
	return json.Marshal(marshalable(ame))
}

func (ame *ActorMoveEvent) UnmarshalJSON(in []byte) error {
	return nil
}

func NewActorAddToZoneEvent(name string, actorId, startingLocationId, zoneId uuid.UUID) ActorAddToZoneEvent {
	return ActorAddToZoneEvent{
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

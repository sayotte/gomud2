package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/rpc"
	myuuid "github.com/sayotte/gomud2/uuid"
)

func NewZone(id uuid.UUID, nickname string, persister EventPersister) *Zone {
	newID := id
	if uuid.Equal(id, uuid.Nil) {
		newID = myuuid.NewId()
	}
	return &Zone{
		id:            newID,
		nickname:      nickname,
		actorsById:    make(map[uuid.UUID]*Actor),
		locationsById: make(map[uuid.UUID]*Location),
		exitsById:     make(map[uuid.UUID]*Exit),
		objectsById:   make(map[uuid.UUID]*Object),
		persister:     persister,
	}
}

type Zone struct {
	id              uuid.UUID
	nickname        string
	world           *World
	defaultLocation *Location
	nextSequenceId  uint64
	actorsById      map[uuid.UUID]*Actor
	locationsById   map[uuid.UUID]*Location
	exitsById       map[uuid.UUID]*Exit
	objectsById     map[uuid.UUID]*Object
	// This is the channel where the Zone picks up new events submitted by its
	// own public methods. This should never be directly exposed by an accessor;
	// public methods should create requests and send them to the channel.
	internalRequestChan chan rpc.Request
	stopChan            chan struct{}
	stopWG              *sync.WaitGroup
	persister           EventPersister
}

//////// getters + non-command-setters

func (z *Zone) ID() uuid.UUID {
	return z.id
}

func (z *Zone) Nickname() string {
	return z.nickname
}

func (z *Zone) Tag() string {
	return strings.Join([]string{z.nickname, z.id.String()}, "/")
}

func (z *Zone) setPersister(ep EventPersister) {
	z.persister = ep
}

func (z *Zone) LastSequenceNum() uint64 {
	return z.nextSequenceId - 1
}

func (z *Zone) Actors() ActorList {
	out := make(ActorList, 0, len(z.actorsById))
	for _, actor := range z.actorsById {
		out = append(out, actor)
	}
	return out
}

func (z *Zone) ActorByID(id uuid.UUID) *Actor {
	return z.actorsById[id]
}

func (z *Zone) Locations() LocationList {
	out := make(LocationList, 0, len(z.locationsById))
	for _, loc := range z.locationsById {
		out = append(out, loc)
	}
	return out
}

func (z *Zone) LocationByID(id uuid.UUID) *Location {
	return z.locationsById[id]
}

func (z *Zone) DefaultLocation() *Location {
	return z.defaultLocation
}

func (z *Zone) Exits() ExitList {
	out := make(ExitList, 0, len(z.exitsById))
	for _, exit := range z.exitsById {
		out = append(out, exit)
	}
	return out
}

func (z *Zone) ExitsToLocation(loc *Location) ExitList {
	var out ExitList
	for _, exit := range z.exitsById {
		if exit.Destination() == loc {
			out = append(out, exit)
		}
	}
	return out
}

func (z *Zone) ObjectByID(id uuid.UUID) *Object {
	return z.objectsById[id]
}

func (z *Zone) World() *World {
	return z.world
}

func (z *Zone) setWorld(world *World) {
	z.world = world
}

//////// public command methods

func (z *Zone) AddActor(a *Actor) (*Actor, error) {
	e := a.snapshot(0).(*ActorAddToZoneEvent)
	cmd := newActorAddToZoneCommand(e)
	val, err := z.syncRequestToSelf(cmd)
	newActor := val.(*Actor)
	return newActor, err
}

func (z *Zone) RemoveActor(a *Actor) error {
	e := NewActorRemoveFromZoneEvent(a.ID(), z.id)
	cmd := newActorRemoveFromZoneCommand(&e)
	_, err := z.syncRequestToSelf(cmd)
	return err
}

func (z *Zone) MigrateInActor(a *Actor, fromLoc, toLoc *Location) (*Actor, error) {
	// We have to do some hijinks with the Observers here, as the new Actor
	// object being created in this zone will not initially have any Observers,
	// so any API/Telnet clients connected to the old object won't see the events
	// generated within this zone.
	//
	// To work around that, we add them temporarily as Observers to the new
	// Location, so they witness the events. After the events are processed,
	// the command-handler will properly add them as Observers to the new Actor,
	// so we can remove them from the Location.

	currentObservers := a.Observers()
	for _, o := range currentObservers {
		toLoc.addObserver(o)
	}
	cmd := newActorMigrateInCommand(a, fromLoc, toLoc, a.Observers())
	out, err := z.syncRequestToSelf(cmd)
	for _, o := range currentObservers {
		toLoc.removeObserver(o)
	}
	return out.(*Actor), err
}

func (z *Zone) MigrateOutActor(a *Actor, fromLoc, toLoc *Location) error {
	cmd := newActorMigrateOutCommand(a, fromLoc, toLoc)
	_, err := z.syncRequestToSelf(cmd)
	return err
}

func (z *Zone) AddLocation(l *Location) (*Location, error) {
	e := l.snapshot(0).(*LocationAddToZoneEvent)
	cmd := newLocationAddToZoneCommand(e)
	val, err := z.syncRequestToSelf(cmd)
	if err != nil {
		return nil, err
	}
	newLoc := val.(*Location)
	return newLoc, nil
}

func (z *Zone) RemoveLocation(l *Location) error {
	e := NewLocationRemoveFromZoneEvent(l.ID(), z.id)
	cmd := newLocationRemoveFromZoneCommand(e)
	_, err := z.syncRequestToSelf(cmd)
	return err
}

func (z *Zone) AddExit(ex *Exit) (*Exit, error) {
	e := ex.snapshot(0).(*ExitAddToZoneEvent)
	cmd := newExitAddToZoneCommand(e)
	val, err := z.syncRequestToSelf(cmd)
	if err != nil {
		return nil, err
	}
	newExit := val.(*Exit)
	return newExit, nil
}

func (z *Zone) RemoveExit(ex *Exit) error {
	e := NewExitRemoveFromZoneEvent(ex.ID(), z.id)
	cmd := newExitRemoveFromZoneCommand(e)
	_, err := z.syncRequestToSelf(cmd)
	return err
}

func (z *Zone) AddObject(o *Object, startingLocation *Location) (*Object, error) {
	e := o.snapshot(0).(*ObjectAddToZoneEvent)
	cmd := newObjectAddToZoneCommand(e)
	val, err := z.syncRequestToSelf(cmd)
	newObject := val.(*Object)
	return newObject, err
}

func (z *Zone) RemoveObject(o *Object) error {
	e := NewObjectRemoveFromZoneEvent(o.Name(), o.ID(), z.id)
	cmd := newObjectRemoveFromZoneCommand(e)
	_, err := z.syncRequestToSelf(cmd)
	return err
}

func (z *Zone) SetDefaultLocation(loc *Location) error {
	e := NewZoneSetDefaultLocationEvent(loc.ID(), z.id)
	cmd := newZoneSetDefaultLocationCommand(e)
	_, err := z.syncRequestToSelf(cmd)
	return err
}

//////// command processing

func (z *Zone) syncRequestToSelf(c Command) (interface{}, error) {
	req := rpc.NewRequest(c)
	z.internalRequestChan <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (z *Zone) StartCommandProcessing() {
	z.internalRequestChan = make(chan rpc.Request)
	z.stopChan = make(chan struct{})
	go func() {
		for {
			// terminate if we're supposed to do that
			select {
			case <-z.stopChan:
				z.stopWG.Done()
				return
			default:
			}

			var requests []rpc.Request

			// absorb *all* outstanding requests from the self-requests channel,
			// which is used for creating new objects / zone transfers etc.
		BreakLoop:
			for {
				select {
				case req := <-z.internalRequestChan:
					requests = append(requests, req)
				default:
					break BreakLoop
				}
			}

			// absorb *one* request from each exit/location/object/actor
			for _, exit := range z.exitsById {
				select {
				case req := <-exit.requestChan:
					requests = append(requests, req)
				default:
				}
			}
			for _, loc := range z.locationsById {
				select {
				case req := <-loc.requestChan:
					requests = append(requests, req)
				default:
				}
			}
			for _, obj := range z.objectsById {
				select {
				case req := <-obj.requestChan:
					requests = append(requests, req)
				default:
				}
			}
			for _, actor := range z.actorsById {
				select {
				case req := <-actor.requestChan:
					requests = append(requests, req)
				default:
				}
			}

			for _, req := range requests {
				e := req.Payload.(Command)
				value, err := z.processCommand(e)
				response := rpc.Response{
					Err:   err,
					Value: value,
				}
				req.ResponseChan <- response
			}

			time.Sleep(time.Second / 8)
		}
	}()
}

func (z *Zone) StopCommandProcessing() {
	if z.stopWG == nil {
		z.stopWG = &sync.WaitGroup{}
	}
	z.stopWG.Add(1)
	close(z.stopChan)
	z.stopWG.Wait()
}

func (z *Zone) processCommand(c Command) (interface{}, error) {
	var outEvents []Event
	var err error
	var out interface{}

	// Command processing happens like so:
	// 1- invoke command handler
	//   1a- create events
	//   1b- apply events
	//      1b1- notify observers of event
	// 2- persist events

	switch c.CommandType() {
	case CommandTypeActorAddToZone:
		out, outEvents, err = z.processActorAddToZoneCommand(c)
	case CommandTypeActorMove:
		outEvents, err = z.processActorMoveCommand(c)
	case CommandTypeActorAdminRelocate:
		outEvents, err = z.processActorAdminRelocateCommand(c)
	case CommandTypeActorRemoveFromZone:
		outEvents, err = z.processActorRemoveCommand(c)
	case CommandTypeActorMigrateIn:
		out, outEvents, err = z.processActorMigrateInCommand(c)
	case CommandTypeActorMigrateOut:
		outEvents, err = z.processActorMigrateOutCommand(c)
	case CommandTypeLocationAddToZone:
		out, outEvents, err = z.processLocationAddToZoneCommand(c)
	case CommandTypeLocationUpdate:
		outEvents, err = z.processLocationUpdateCommand(c)
	case CommandTypeLocationRemoveFromZone:
		outEvents, err = z.processLocationRemoveFromZoneCommand(c)
	case CommandTypeExitAddToZone:
		out, outEvents, err = z.processExitAddToZoneCommand(c)
	case CommandTypeExitUpdate:
		outEvents, err = z.processExitUpdateCommand(c)
	case CommandTypeExitRemoveFromZone:
		outEvents, err = z.processExitRemoveFromZoneCommand(c)
	case CommandTypeObjectAddToZone:
		out, outEvents, err = z.processObjectAddToZoneCommand(c)
	case CommandTypeObjectMove:
		outEvents, err = z.processObjectMoveCommand(c)
	case CommandTypeObjectAdminRelocate:
		outEvents, err = z.processObjectAdminRelocateCommand(c)
	case CommandTypeObjectRemoveFromZone:
		outEvents, err = z.processExitRemoveFromZoneCommand(c)
	case CommandTypeZoneSetDefaultLocation:
		outEvents, err = z.processZoneSetDefaultLocationCommand(c)
	default:
		err = fmt.Errorf("unrecognized Command type %d", c.CommandType())
	}
	if err != nil {
		return nil, err
	}

	for _, e := range outEvents {
		if z.persister != nil {
			err = z.persister.PersistEvent(e)
		}
	}

	return out, err
}

func (z *Zone) processActorAddToZoneCommand(c Command) (interface{}, []Event, error) {
	cmd := c.(actorAddToZoneCommand)
	e := cmd.wrappedEvent

	_, ok := z.locationsById[e.startingLocationId]
	if !ok {
		return nil, nil, fmt.Errorf("unknown startingLocation %q", e.startingLocationId)
	}
	_, duplicate := z.actorsById[e.actorId]
	if duplicate {
		return nil, nil, fmt.Errorf("Actor %q already present in zone", e.actorId)
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	out, err := z.applyEvent(e)
	return out, []Event{e}, err
}

func (z *Zone) processActorMoveCommand(c Command) ([]Event, error) {
	cmd := c.(actorMoveCommand)
	e := cmd.wrappedEvent

	from, ok := z.locationsById[e.fromLocationId]
	if !ok {
		return nil, fmt.Errorf("unknown from-location %q", e.fromLocationId)
	}
	to, ok := z.locationsById[e.toLocationId]
	if !ok {
		return nil, fmt.Errorf("unknown to-location %q", e.toLocationId)
	}
	exitExists := false
	for _, exit := range from.OutExits() {
		if exit.Destination() == to {
			exitExists = true
			break
		}
	}
	if !exitExists {
		return nil, fmt.Errorf("no exit to that destination from location %q", from.ID())
	}
	_, ok = z.actorsById[e.actorId]
	if !ok {
		return nil, fmt.Errorf("unknown Actor %q", e.actorId)
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	_, err := z.applyEvent(e)
	return []Event{e}, err
}

func (z *Zone) processActorAdminRelocateCommand(c Command) ([]Event, error) {
	cmd := c.(actorAdminRelocateCommand)
	e := cmd.wrappedEvent

	_, found := z.actorsById[e.ActorID]
	if !found {
		return nil, fmt.Errorf("no such Actor with ID %q in Zone", e.ActorID)
	}
	_, found = z.locationsById[e.ToLocationID]
	if !found {
		return nil, fmt.Errorf("no such Location with ID %q in Zone", e.ToLocationID)
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	_, err := z.applyEvent(e)
	return []Event{e}, err
}

func (z *Zone) processActorRemoveCommand(c Command) ([]Event, error) {
	cmd := c.(actorRemoveFromZoneCommand)
	e := cmd.wrappedEvent
	_, found := z.actorsById[e.actorID]
	if !found {
		return nil, fmt.Errorf("Actor %q not found in Zone", e.actorID)
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	_, err := z.applyEvent(e)
	return []Event{e}, err
}

func (z *Zone) processActorMigrateInCommand(c Command) (interface{}, []Event, error) {
	cmd := c.(*actorMigrateInCommand)

	var outEvents []Event

	actorEv := NewActorMigrateInEvent(
		cmd.actor.Name(),
		cmd.actor.ID(),
		cmd.from.ID(),
		cmd.from.Zone().ID(),
		cmd.to.ID(),
		z.id,
	)
	actorEv.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = actorEv.SequenceNumber() + 1
	out, err := z.applyEvent(actorEv)
	if err != nil {
		return nil, nil, err
	}
	newActor := out.(*Actor)
	for _, o := range cmd.observers {
		newActor.AddObserver(o)
	}
	outEvents = []Event{actorEv}

	for _, objContTuple := range getObjectContainerTuplesRecursive(cmd.actor) {
		var locContID, actorContID, objContID uuid.UUID
		switch objContTuple.cont.(type) {
		case *Location:
			locContID = objContTuple.cont.ID()
		case *Actor:
			actorContID = objContTuple.cont.ID()
		case *Object:
			objContID = objContTuple.cont.ID()
		}
		objEv := NewObjectMigrateInEvent(
			objContTuple.obj.Name(),
			objContTuple.obj.Keywords(),
			objContTuple.obj.Capacity(),
			objContTuple.obj.ID(),
			cmd.from.Zone().ID(),
			locContID,
			actorContID,
			objContID,
			z.id,
		)
		objEv.SetSequenceNumber(z.nextSequenceId)
		z.nextSequenceId = objEv.SequenceNumber() + 1
		_, err := z.applyEvent(objEv)
		if err != nil {
			return nil, nil, err
		}
		outEvents = append(outEvents, objEv)
	}

	return newActor, outEvents, nil
}

func (z *Zone) processActorMigrateOutCommand(c Command) ([]Event, error) {
	cmd := c.(*actorMigrateOutCommand)

	var outEvents []Event

	for _, objContTuple := range getObjectContainerTuplesRecursive(cmd.actor) {
		objEv := NewObjectMigrateOutEvent(
			objContTuple.obj.Name(),
			objContTuple.obj.ID(),
			cmd.to.Zone().ID(),
			z.id,
		)
		objEv.SetSequenceNumber(z.nextSequenceId)
		z.nextSequenceId = objEv.SequenceNumber() + 1
		_, err := z.applyEvent(objEv)
		if err != nil {
			return nil, err
		}
		outEvents = append(outEvents, objEv)
	}

	actorEv := NewActorMigrateOutEvent(
		cmd.actor.ID(),
		cmd.from.ID(),
		cmd.to.ID(),
		cmd.to.Zone().ID(),
		z.id,
	)
	actorEv.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = actorEv.SequenceNumber() + 1
	_, err := z.applyEvent(actorEv)
	if err != nil {
		return nil, err
	}
	outEvents = append(outEvents, actorEv)

	return outEvents, nil
}

func (z *Zone) processLocationAddToZoneCommand(c Command) (interface{}, []Event, error) {
	cmd := c.(locationAddToZoneCommand)
	e := cmd.wrappedEvent
	_, duplicate := z.locationsById[e.locationId]
	if duplicate {
		return nil, nil, fmt.Errorf("Location with ID %q already present in Zone", e.locationId)
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	loc, err := z.applyEvent(e)
	return loc, []Event{e}, err
}

func (z *Zone) processLocationUpdateCommand(c Command) ([]Event, error) {
	cmd := c.(locationUpdateCommand)
	e := cmd.wrappedEvent

	_, ok := z.locationsById[e.locationID]
	if !ok {
		return nil, fmt.Errorf("unknown Location %q", e.locationID)
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	_, err := z.applyEvent(e)
	return []Event{e}, err
}

func (z *Zone) processLocationRemoveFromZoneCommand(c Command) ([]Event, error) {
	cmd := c.(locationRemoveFromZoneCommand)
	e := cmd.wrappedEvent

	loc, found := z.locationsById[e.LocationID]
	if !found {
		return nil, fmt.Errorf("no Location with ID %q found in Zone", e.LocationID)
	}
	if z.defaultLocation == nil {
		if len(loc.Actors()) != 0 || len(loc.Objects()) != 0 {
			return nil, fmt.Errorf("")
		}
	}

	var outEvents []Event

	// First remove all exits
	for _, outExit := range append(loc.OutExits(), z.ExitsToLocation(loc)...) {
		e := NewExitRemoveFromZoneEvent(outExit.ID(), z.id)
		subCmd := newExitRemoveFromZoneCommand(e)
		newEvents, err := z.processExitRemoveFromZoneCommand(subCmd)
		if err != nil {
			return nil, err
		}
		outEvents = append(outEvents, newEvents...)
	}
	// Relocate Actors and Objects so they aren't orphaned
	for _, actor := range loc.Actors() {
		e := NewActorAdminRelocateEvent(actor.id, z.defaultLocation.ID(), z.id)
		subCmd := newActorAdminRelocateCommand(e)
		newEvents, err := z.processActorAdminRelocateCommand(subCmd)
		if err != nil {
			return nil, err
		}
		outEvents = append(outEvents, newEvents...)
	}
	for _, object := range loc.Objects() {
		e := NewObjectAdminRelocateEvent(object.id, z.id)
		e.ToLocationContainerID = z.defaultLocation.ID()
		subCmd := newObjectAdminRelocateCommand(e)
		newEvents, err := z.processObjectAdminRelocateCommand(subCmd)
		if err != nil {
			return nil, err
		}
		outEvents = append(outEvents, newEvents...)
	}

	// Remove the empty, unlinked Location
	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	_, err := z.applyEvent(e)
	return []Event{e}, err
}

func (z *Zone) processExitAddToZoneCommand(c Command) (interface{}, []Event, error) {
	cmd := c.(exitAddToZoneCommand)
	e := cmd.wrappedEvent

	_, duplicate := z.exitsById[e.ExitID]
	if duplicate {
		return nil, nil, fmt.Errorf("Exit with ID %q already present in zone", e.ExitID)
	}
	srcLoc, ok := z.locationsById[e.SourceLocationId]
	if !ok {
		return nil, nil, fmt.Errorf("unknown source location %q", e.SourceLocationId)
	}
	for _, existingExit := range srcLoc.OutExits() {
		if existingExit.Direction() == e.Direction {
			return nil, nil, fmt.Errorf("Exit in direction %q already exists from Location", e.Direction)
		}
	}
	if uuid.Equal(e.DestZoneID, uuid.Nil) {
		var ok bool
		_, ok = z.locationsById[e.DestLocationId]
		if !ok {
			return nil, nil, fmt.Errorf("unknown destination Location %q", e.DestLocationId)
		}
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	newExit, err := z.applyEvent(e)
	return newExit, []Event{e}, err
}

func (z *Zone) processExitUpdateCommand(c Command) ([]Event, error) {
	cmd := c.(exitUpdateCommand)
	e := cmd.wrappedEvent

	// Check for all invalid cases / dereferences
	exit, ok := z.exitsById[e.ExitID]
	if !ok {
		return nil, fmt.Errorf("unknown Exit %q", e.ExitID)
	}
	_, ok = z.locationsById[e.SourceLocationId]
	if !ok {
		return nil, fmt.Errorf("unknown source Location %q", e.SourceLocationId)
	}
	if uuid.Equal(e.DestLocationId, uuid.Nil) {
		return nil, errors.New("destination Location ID cannot be nil")
	}
	var _ *Location
	if uuid.Equal(e.DestZoneID, uuid.Nil) {
		_, ok := z.locationsById[e.DestLocationId]
		if !ok {
			return nil, fmt.Errorf("unknown destination Location %q", e.DestLocationId)
		}
	}
	// Truth table for destination updates
	//| old otherZoneID | old otherZoneLocID | old Dest* | new otherZoneID | action                      | note                                               |
	//|-----------------+--------------------+-----------+-----------------+-----------------------------+----------------------------------------------------|
	//| nil             | nil                | non-nil   | nil             | resolve destination         | internal -> internal                               |
	//|                 |                    |           |                 | exit.setDestination()       |                                                    |
	//|                 |                    |           |                 |                             |                                                    |
	//| nil             | nil                | non-nil   | non-nil         | exit.setDestination(nil)    | internal -> external                               |
	//|                 |                    |           |                 | exit.setOtherZoneID()       |                                                    |
	//|                 |                    |           |                 | exit.setotherZoneLocID()    |                                                    |
	//|                 |                    |           |                 |                             |                                                    |
	//| non-nil         | non-nil            | nil       | nil             | resolve destination         | external -> internal                               |
	//|                 |                    |           |                 | exit.setDestination()       |                                                    |
	//|                 |                    |           |                 | exit.setOtherZoneID(nil)    |                                                    |
	//|                 |                    |           |                 | exit.setOtherZoneLocID(nil) |                                                    |
	//|                 |                    |           |                 |                             |                                                    |
	//| non-nil         | non-nil            | nil       | non-nil         | exit.setOtherZoneID()       | external -> external                               |
	//|                 |                    |           |                 | exit.setOtherZoneLocID()    |                                                    |
	//|                 |                    |           |                 |                             |                                                    |
	//| nil             | nil                | nil       | *               | error                       | invalid prior state, no destination                |
	//| non-nil         | non-nil            | non-nil   | *               | error                       | invalid prior state, internal+external destination |
	//| nil             | non-nil            | *         | *               | error                       | invalid prior state, external loc ref w/o zone ref |
	//| non-nil         | nil                | *         | *               | error                       | invalid prior state, external zone ref w/o loc ref |
	///// Handle destination error cases
	if uuid.Equal(exit.OtherZoneID(), uuid.Nil) && uuid.Equal(exit.OtherZoneLocID(), uuid.Nil) && exit.Destination() == nil {
		return nil, errors.New("invalid prior state, Exit has no internal/external destination")
	}
	if !uuid.Equal(exit.OtherZoneID(), uuid.Nil) && !uuid.Equal(exit.OtherZoneLocID(), uuid.Nil) && exit.Destination() != nil {
		return nil, errors.New("invalid prior state, Exit has both internal/external destinations")
	}
	if uuid.Equal(exit.OtherZoneID(), uuid.Nil) && !uuid.Equal(exit.OtherZoneLocID(), uuid.Nil) {
		return nil, errors.New("invalid prior state, Exit has external Location reference without Zone reference")
	}
	if !uuid.Equal(exit.OtherZoneID(), uuid.Nil) && uuid.Equal(exit.OtherZoneLocID(), uuid.Nil) {
		return nil, errors.New("invalid prior state, Exit has external Zone reference without Location reference")
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	_, err := z.applyEvent(e)
	return []Event{e}, err
}

func (z *Zone) processExitRemoveFromZoneCommand(c Command) ([]Event, error) {
	cmd := c.(exitRemoveFromZoneCommand)
	e := cmd.wrappedEvent

	_, found := z.exitsById[e.ExitID]
	if !found {
		return nil, fmt.Errorf("no such Exit with ID %q in Zone", e.ExitID)
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	_, err := z.applyEvent(e)
	return []Event{e}, err
}

func (z *Zone) processObjectAddToZoneCommand(c Command) (interface{}, []Event, error) {
	cmd := c.(objectAddToZoneCommand)
	e := cmd.wrappedEvent

	_, duplicate := z.objectsById[e.ObjectID]
	if duplicate {
		return nil, nil, fmt.Errorf("Object with ID %q already present in zone", e.ObjectID)
	}

	var found bool
	switch {
	case !uuid.Equal(e.LocationContainerID, uuid.Nil):
		_, found = z.locationsById[e.LocationContainerID]
	case !uuid.Equal(e.ActorContainerID, uuid.Nil):
		_, found = z.actorsById[e.ActorContainerID]
	case !uuid.Equal(e.ObjectContainerID, uuid.Nil):
		_, found = z.objectsById[e.ObjectContainerID]
	}
	if !found {
		return nil, nil, fmt.Errorf("no resolvable container")
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	out, err := z.applyEvent(e)
	return out, []Event{e}, err
}

func (z *Zone) processObjectMoveCommand(c Command) ([]Event, error) {
	cmd := c.(objectMoveCommand)
	_, found := z.objectsById[cmd.obj.ID()]
	if !found || cmd.obj.Zone() != z {
		return nil, errors.New("Object not found / this Zone cannot move objects in other Zones")
	}

	if cmd.fromContainer == cmd.toContainer {
		return nil, errors.New("'from' and 'to' Containers are the same")
	}

	if !cmd.fromContainer.ContainsObject(cmd.obj) {
		return nil, errors.New("'from' Container does not currently contain Object")
	}

	if cmd.fromContainer.Location() != cmd.toContainer.Location() {
		return nil, errors.New("cannot move an Object directly between Containers in different Locations")
	}

	if len(cmd.fromContainer.Objects()) >= cmd.fromContainer.Capacity() {
		return nil, errors.New("would overflow container")
	}

	var actorID uuid.UUID
	if cmd.actor != nil {
		actorID = cmd.actor.ID()
	}
	e := NewObjectMoveEvent(cmd.obj.ID(), actorID, z.id)

	fromID := cmd.fromContainer.ID()
	switch cmd.fromContainer.(type) {
	case *Location:
		_, found := z.locationsById[fromID]
		if !found {
			return nil, fmt.Errorf("unknown 'from' Container/Location %q", fromID)
		}
		e.FromLocationContainerID = fromID
	case *Actor:
		_, found := z.actorsById[fromID]
		if !found {
			return nil, fmt.Errorf("unknown 'from' Container/Actor %q", fromID)
		}
		e.FromActorContainerID = fromID
	case *Object:
		_, found := z.objectsById[fromID]
		if !found {
			return nil, fmt.Errorf("unknown 'from' Container/Object %q", fromID)
		}
		e.FromObjectContainerID = fromID
	default:
		return nil, fmt.Errorf("don't know how to handle 'from' Container type %T", cmd.fromContainer)
	}

	toID := cmd.toContainer.ID()
	switch cmd.toContainer.(type) {
	case *Location:
		_, found := z.locationsById[toID]
		if !found {
			return nil, fmt.Errorf("unknown 'to' Container/Location %q", toID)
		}
		e.ToLocationContainerID = toID
	case *Actor:
		_, found := z.actorsById[toID]
		if !found {
			return nil, fmt.Errorf("unknown 'to' Container/Actor %q", toID)
		}
		e.ToActorContainerID = toID
	case *Object:
		_, found := z.objectsById[toID]
		if !found {
			return nil, fmt.Errorf("unknown 'to' Container/Object %q", toID)
		}
		e.ToObjectContainerID = toID
	default:
		return nil, fmt.Errorf("don't know how to handle 'to' Container type %T", cmd.toContainer)
	}

	if uuid.Equal(e.FromActorContainerID, uuid.Nil) && uuid.Equal(e.ToActorContainerID, uuid.Nil) {
		return nil, errors.New("illegal Object movement, must be to/from an Actor")
	}

	if !uuid.Equal(e.FromActorContainerID, uuid.Nil) && !uuid.Equal(e.ToActorContainerID, uuid.Nil) {
		// actor -> actor movement, and recipient actor is also the one doing the movement
		if uuid.Equal(e.ToActorContainerID, e.ActorID) {
			return nil, errors.New("illegal Object movement, stealing not allowed")
		}
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	_, err := z.applyEvent(e)
	return []Event{e}, err
}

func (z *Zone) processObjectAdminRelocateCommand(c Command) ([]Event, error) {
	cmd := c.(objectAdminRelocateCommand)
	e := cmd.wrappedEvent

	var found bool
	_, found = z.objectsById[e.ObjectID]
	if !found {
		return nil, fmt.Errorf("no such Object with ID %q in Zone", e.ObjectID)
	}

	switch {
	case !uuid.Equal(e.ToLocationContainerID, uuid.Nil):
		_, found = z.locationsById[e.ToLocationContainerID]
	case !uuid.Equal(e.ToActorContainerID, uuid.Nil):
		_, found = z.actorsById[e.ToActorContainerID]
	case !uuid.Equal(e.ToObjectContainerID, uuid.Nil):
		_, found = z.objectsById[e.ToObjectContainerID]
	}
	if !found {
		return nil, fmt.Errorf("no resolvable to-Container ID")
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	_, err := z.applyEvent(e)
	return []Event{e}, err
}

func (z *Zone) processObjectRemoveFromZoneCommand(c Command) ([]Event, error) {
	cmd := c.(objectRemoveFromZoneCommand)
	e := cmd.wrappedEvent

	_, found := z.objectsById[e.ObjectID]
	if !found {
		return nil, fmt.Errorf("Object %q not found in Zone", e.ObjectID)
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	_, err := z.applyEvent(e)
	return []Event{e}, err
}

func (z *Zone) processZoneSetDefaultLocationCommand(c Command) ([]Event, error) {
	cmd := c.(zoneSetDefaultLocationCommand)
	e := cmd.wrappedEvent

	_, ok := z.locationsById[e.LocationID]
	if !ok {
		return nil, fmt.Errorf("no such Location with ID %q in Zone", e.LocationID)
	}

	e.SetSequenceNumber(z.nextSequenceId)
	z.nextSequenceId = e.SequenceNumber() + 1
	_, err := z.applyEvent(e)
	return []Event{e}, err
}

//////// Event processing

func (z *Zone) sendEventToObservers(e Event, oList ObserverList) {
	for _, o := range oList {
		o.SendEvent(e)
	}
}

// this is used to rebuild state from an Event store; it is not to be used
// during normal operations
func (z *Zone) ReplayEvents(inChan <-chan rpc.Response) error {
	for res := range inChan {
		if res.Err != nil {
			return res.Err
		}
		_, err := z.applyEvent(res.Value.(Event))
		if err != nil {
			return err
		}
	}
	return nil
}

func (z *Zone) applyEvent(e Event) (interface{}, error) {
	fmt.Printf("DEBUG: Zone applying event %d\n", e.SequenceNumber())

	var out interface{}
	var oList ObserverList
	var err error

	switch e.Type() {
	case EventTypeActorAddToZone:
		typedEvent := e.(*ActorAddToZoneEvent)
		out, oList, err = z.applyActorAddToZoneEvent(typedEvent)
	case EventTypeActorMove:
		typedEvent := e.(*ActorMoveEvent)
		oList, err = z.applyActorMoveEvent(typedEvent)
	case EventTypeActorAdminRelocate:
		typedEvent := e.(*ActorAdminRelocateEvent)
		oList, err = z.applyActorAdminRelocateEvent(typedEvent)
	case EventTypeActorRemoveFromZone:
		typedEvent := e.(*ActorRemoveFromZoneEvent)
		oList, err = z.applyActorRemoveEvent(typedEvent)
	case EventTypeActorMigrateIn:
		typedEvent := e.(*ActorMigrateInEvent)
		out, oList, err = z.applyActorMigrateInEvent(typedEvent)
	case EventTypeActorMigrateOut:
		typedEvent := e.(*ActorMigrateOutEvent)
		oList, err = z.applyActorMigrateOutEvent(typedEvent)
	case EventTypeLocationAddToZone:
		typedEvent := e.(*LocationAddToZoneEvent)
		out, err = z.applyLocationAddToZoneEvent(typedEvent)
	case EventTypeLocationUpdate:
		typedEvent := e.(*LocationUpdateEvent)
		err = z.applyLocationUpdateEvent(typedEvent)
	case EventTypeLocationRemoveFromZone:
		typedEvent := e.(*LocationRemoveFromZoneEvent)
		z.applyLocationRemoveFromZoneEvent(typedEvent)
	case EventTypeExitAddToZone:
		typedEvent := e.(*ExitAddToZoneEvent)
		out, err = z.applyExitAddToZoneEvent(typedEvent)
	case EventTypeExitUpdate:
		typedEvent := e.(*ExitUpdateEvent)
		err = z.applyExitUpdateEvent(typedEvent)
	case EventTypeExitRemoveFromZone:
		typedEvent := e.(*ExitRemoveFromZoneEvent)
		err = z.applyExitRemoveFromZoneEvent(typedEvent)
	case EventTypeObjectAddToZone:
		typedEvent := e.(*ObjectAddToZoneEvent)
		out, oList, err = z.applyObjectAddToZoneEvent(typedEvent)
	case EventTypeObjectMove:
		typedEvent := e.(*ObjectMoveEvent)
		oList, err = z.applyObjectMoveEvent(typedEvent)
	case EventTypeObjectAdminRelocate:
		typedEvent := e.(*ObjectAdminRelocateEvent)
		oList, err = z.applyObjectAdminRelocateEvent(typedEvent)
	case EventTypeObjectRemoveFromZone:
		typedEvent := e.(*ObjectRemoveFromZoneEvent)
		oList, err = z.applyObjectRemoveFromZoneEvent(typedEvent)
	case EventTypeObjectMigrateIn:
		typedEvent := e.(*ObjectMigrateInEvent)
		err = z.applyObjectMigrateInEvent(typedEvent)
	case EventTypeObjectMigrateOut:
		typedEvent := e.(*ObjectMigrateOutEvent)
		err = z.applyObjectMigrateOutEvent(typedEvent)
	case EventTypeZoneSetDefaultLocation:
		typedEvent := e.(*ZoneSetDefaultLocationEvent)
		err = z.applyZoneSetDefaultLocationEvent(typedEvent)
	default:
		err = fmt.Errorf("unknown Event type %T", e)
	}

	if err != nil {
		return nil, err
	}

	z.nextSequenceId = e.SequenceNumber() + 1

	z.sendEventToObservers(e, oList)
	return out, nil
}

func (z *Zone) applyActorAddToZoneEvent(e *ActorAddToZoneEvent) (*Actor, ObserverList, error) {
	newLoc := z.locationsById[e.startingLocationId]
	if newLoc == nil {
		if z.defaultLocation == nil {
			return nil, nil, fmt.Errorf("cannot resolve StartingLocationID %q, and no default Location set for Zone", e.startingLocationId)
		}
		newLoc = z.defaultLocation
	}
	actor := NewActor(e.ActorID(), e.name, newLoc, z)

	newLoc.addActor(actor)
	actor.setLocation(newLoc)
	z.actorsById[actor.ID()] = actor

	oList := newLoc.Observers()

	return actor, oList, nil
}

func (z *Zone) applyActorMoveEvent(e *ActorMoveEvent) (ObserverList, error) {
	fromLoc := z.locationsById[e.fromLocationId]
	toLoc := z.locationsById[e.toLocationId]
	actor := z.actorsById[e.actorId]

	if fromLoc == nil || toLoc == nil || actor == nil {
		return nil, errors.New("cannot resolve From/To/Actor IDs in event")
	}

	fromLoc.removeActor(actor)
	toLoc.addActor(actor)
	actor.setLocation(toLoc)

	var oList ObserverList
	oList = actor.Observers()
	for _, o := range fromLoc.Observers() {
		oList = append(oList, o)
	}
	for _, o := range toLoc.Observers() {
		oList = append(oList, o)
	}

	return oList, nil
}

func (z *Zone) applyActorAdminRelocateEvent(e *ActorAdminRelocateEvent) (ObserverList, error) {
	actor := z.actorsById[e.ActorID]
	toLoc := z.locationsById[e.ToLocationID]
	if actor == nil || toLoc == nil {
		return nil, errors.New("cannot resolve Actor and to-Location IDs")
	}

	var oList ObserverList
	// It's possible the Actor has been orphaned with no location, which is why we're
	// relocating them. Otherwise, though, we need to remove them from their existing
	// location.
	fromLoc := actor.Location()
	if fromLoc != nil {
		fromLoc.removeActor(actor)
		oList = fromLoc.Observers()
	}
	toLoc.addActor(actor)
	actor.setLocation(toLoc)
	oList = append(oList, toLoc.Observers()...)

	return oList, nil
}

func (z *Zone) applyActorRemoveEvent(e *ActorRemoveFromZoneEvent) (ObserverList, error) {
	actor := z.actorsById[e.actorID]
	if actor == nil {
		return nil, errors.New("cannot find Actor to remove")
	}

	oldLoc := actor.Location()
	oldLoc.removeActor(actor)
	actor.setLocation(nil)
	actor.setZone(nil)
	delete(z.actorsById, e.actorID)

	oList := oldLoc.Observers()

	return oList, nil
}

func (z *Zone) applyActorMigrateInEvent(e *ActorMigrateInEvent) (*Actor, ObserverList, error) {
	newLoc := z.locationsById[e.ToLocID]
	actor := NewActor(e.ActorID, e.Name, newLoc, z)

	var oList ObserverList
	if newLoc != nil {
		newLoc.addActor(actor)
		oList = newLoc.Observers()
	} else {
		fmt.Printf("WARNING: processing ActorMigrateInEvent with no resolvable Location\n")
	}
	actor.setLocation(newLoc)
	z.actorsById[actor.ID()] = actor

	return actor, oList, nil
}

func (z *Zone) applyActorMigrateOutEvent(e *ActorMigrateOutEvent) (ObserverList, error) {
	actor, found := z.actorsById[e.ActorID]
	if !found {
		return nil, fmt.Errorf("cannot find Actor %q to migrate out of Zone", e.ActorID)
	}

	oList := append(actor.Observers(), actor.Location().Observers()...)

	actor.Location().removeActor(actor)
	actor.setLocation(nil)
	actor.setZone(nil)
	delete(z.actorsById, actor.ID())

	return oList, nil
}

func (z *Zone) applyLocationAddToZoneEvent(e *LocationAddToZoneEvent) (*Location, error) {
	loc := NewLocation(
		e.locationId,
		z,
		e.shortDesc,
		e.desc,
	)
	z.locationsById[e.locationId] = loc
	return loc, nil
}

func (z *Zone) applyLocationUpdateEvent(e *LocationUpdateEvent) error {
	loc, ok := z.locationsById[e.locationID]
	if !ok {
		return fmt.Errorf("unknown Location %q", e.locationID)
	}

	loc.setShortDescription(e.ShortDescription())
	loc.setDescription(e.Description())
	return nil
}

func (z *Zone) applyLocationRemoveFromZoneEvent(e *LocationRemoveFromZoneEvent) {
	delete(z.locationsById, e.LocationID)
}

func (z *Zone) applyExitAddToZoneEvent(e *ExitAddToZoneEvent) (*Exit, error) {
	srcLoc, ok := z.locationsById[e.SourceLocationId]
	if !ok {
		return nil, fmt.Errorf("unknown source location %q", e.SourceLocationId)
	}
	var destLoc *Location   // non-nil if same zone, nil if different zone
	var destLocID uuid.UUID // non-nil if different zone, nil if same zone
	if uuid.Equal(e.DestZoneID, uuid.Nil) {
		var ok bool
		destLoc, ok = z.locationsById[e.DestLocationId]
		if !ok {
			return nil, fmt.Errorf("unknown destination location %q", e.DestLocationId)
		}
	} else {
		destLocID = e.DestLocationId
	}

	exit := NewExit(
		e.ExitID,
		e.Description,
		e.Direction,
		srcLoc,
		destLoc,
		z,
		e.DestZoneID,
		destLocID,
	)
	err := srcLoc.addOutExit(exit)
	if err != nil {
		return nil, err
	}
	z.exitsById[exit.ID()] = exit
	return exit, nil
}

func (z *Zone) applyExitUpdateEvent(e *ExitUpdateEvent) error {
	// Do dereferences
	exit, ok := z.exitsById[e.ExitID]
	if !ok {
		return fmt.Errorf("unknown Exit %q", e.ExitID)
	}
	newSrc, ok := z.locationsById[e.SourceLocationId]
	if !ok {
		return fmt.Errorf("unknown source Location %q", e.SourceLocationId)
	}
	if uuid.Equal(e.DestLocationId, uuid.Nil) {
		return errors.New("destination Location ID cannot be nil")
	}
	var newDst *Location
	if uuid.Equal(e.DestZoneID, uuid.Nil) {
		_, ok := z.locationsById[e.DestLocationId]
		if !ok {
			return fmt.Errorf("unknown destination Location %q", e.DestLocationId)
		}
	}

	// Handle destination update cases
	if uuid.Equal(exit.OtherZoneID(), uuid.Nil) {
		if uuid.Equal(e.DestZoneID, uuid.Nil) {
			// internal -> internal
			exit.setDestination(newDst)
		} else {
			// internal -> external
			exit.setDestination(nil)
			exit.setOtherZoneID(e.DestZoneID)
			exit.setOtherZoneLocID(e.DestLocationId)
		}
	} else {
		if uuid.Equal(e.DestZoneID, uuid.Nil) {
			// external -> internal
			exit.setDestination(newDst)
			exit.setOtherZoneID(uuid.Nil)
			exit.setOtherZoneLocID(uuid.Nil)
		} else {
			// external -> external
			exit.setOtherZoneID(e.DestZoneID)
			exit.setOtherZoneLocID(e.DestLocationId)
		}
	}

	// Changing source location requires updating bi-directional pointers
	if newSrc != exit.Source() {
		oldSrc := exit.Source()
		oldSrc.removeExit(exit)
		err := newSrc.addOutExit(exit)
		if err != nil {
			return err
		}
		exit.setSource(newSrc)
	}

	exit.setDescription(e.Description)
	exit.setDirection(e.Direction)

	return nil
}

func (z *Zone) applyExitRemoveFromZoneEvent(e *ExitRemoveFromZoneEvent) error {
	ex, found := z.exitsById[e.ExitID]
	if !found {
		return fmt.Errorf("no such Exit with ID %q in Zone", e.ExitID)
	}

	ex.Source().removeExit(ex)
	delete(z.exitsById, e.ExitID)

	return nil
}

func (z *Zone) applyObjectAddToZoneEvent(e *ObjectAddToZoneEvent) (*Object, ObserverList, error) {
	var container Container
	var oList ObserverList

	switch {
	case !uuid.Equal(e.LocationContainerID, uuid.Nil):
		container = z.locationsById[e.LocationContainerID]
	case !uuid.Equal(e.ActorContainerID, uuid.Nil):
		container = z.actorsById[e.ActorContainerID]
	case !uuid.Equal(e.ObjectContainerID, uuid.Nil):
		container = z.objectsById[e.ObjectContainerID]
	}

	obj := NewObject(e.ObjectID, e.Name, e.Keywords, container, e.Capacity, z)
	if container != nil {
		err := container.addObject(obj)
		if err != nil {
			return nil, nil, err
		}
		oList = container.Observers()
	} else {
		fmt.Printf("WARNING: processing ObjectAddToZoneEvent with no resolvable Container\n")
	}

	obj.setContainer(container)
	z.objectsById[obj.ID()] = obj

	return obj, oList, nil
}

func (z *Zone) applyObjectMoveEvent(e *ObjectMoveEvent) (ObserverList, error) {
	var from, to Container
	var oList ObserverList

	obj, found := z.objectsById[e.ObjectID]
	if !found {
		return nil, fmt.Errorf("cannot move non-existent object %q", e.ObjectID)
	}

	switch {
	case !uuid.Equal(e.FromLocationContainerID, uuid.Nil):
		from = z.locationsById[e.FromLocationContainerID]
	case !uuid.Equal(e.FromActorContainerID, uuid.Nil):
		from = z.actorsById[e.FromActorContainerID]
	case !uuid.Equal(e.FromObjectContainerID, uuid.Nil):
		from = z.objectsById[e.FromObjectContainerID]
	}

	switch {
	case !uuid.Equal(e.ToLocationContainerID, uuid.Nil):
		to = z.locationsById[e.ToLocationContainerID]
	case !uuid.Equal(e.ToActorContainerID, uuid.Nil):
		to = z.actorsById[e.ToActorContainerID]
	case !uuid.Equal(e.ToObjectContainerID, uuid.Nil):
		to = z.objectsById[e.ToObjectContainerID]
	}

	if from != nil {
		from.removeObject(obj)
		oList = from.Observers()
	} else {
		fmt.Printf("WARNING: applying ObjectMoveEvent with no resolvable 'from'\n")
	}

	if to != nil {
		_ = to.addObject(obj)
		oList = append(oList, to.Observers()...)
	} else {
		fmt.Printf("WARNING: applying ObjectMoveEvent with no resolvable 'to'\n")
	}

	obj.setContainer(to)

	oList = oList.Dedupe()

	return oList, nil
}

func (z *Zone) applyObjectAdminRelocateEvent(e *ObjectAdminRelocateEvent) (ObserverList, error) {
	obj, found := z.objectsById[e.ObjectID]
	if !found {
		return nil, fmt.Errorf("no such Object with ID %q in Zone", e.ObjectID)
	}
	var to Container
	switch {
	case !uuid.Equal(e.ToLocationContainerID, uuid.Nil):
		to = z.locationsById[e.ToLocationContainerID]
	case !uuid.Equal(e.ToActorContainerID, uuid.Nil):
		to = z.actorsById[e.ToActorContainerID]
	case !uuid.Equal(e.ToObjectContainerID, uuid.Nil):
		to = z.objectsById[e.ToObjectContainerID]
	}
	if to == nil {
		return nil, fmt.Errorf("no resolvable to-Container")
	}

	var oList ObserverList
	// It's possible the Object has been orphaned with no container, which is why we're
	// relocating them. Otherwise, though, we need to remove them from their existing
	// container.
	from := obj.Container()
	if from != nil {
		from.removeObject(obj)
		oList = from.Observers()
	}
	err := to.addObject(obj)
	if err != nil {
		return nil, err
	}
	obj.setContainer(to)
	oList = append(oList, to.Observers()...)

	return oList, nil
}

func (z *Zone) applyObjectRemoveFromZoneEvent(e *ObjectRemoveFromZoneEvent) (ObserverList, error) {
	object, found := z.objectsById[e.ObjectID]
	if !found {
		return nil, fmt.Errorf("Object %q not found in Zone", e.ObjectID)
	}
	oldContainer := object.Container()
	oldContainer.removeObject(object)
	object.setContainer(nil)
	object.setZone(nil)
	delete(z.objectsById, object.ID())

	oList := oldContainer.Observers()

	return oList, nil
}

func (z *Zone) applyObjectMigrateInEvent(e *ObjectMigrateInEvent) error {
	var container Container

	switch {
	case !uuid.Equal(e.LocationContainerID, uuid.Nil):
		container = z.locationsById[e.LocationContainerID]
	case !uuid.Equal(e.ActorContainerID, uuid.Nil):
		container = z.actorsById[e.ActorContainerID]
	case !uuid.Equal(e.ObjectContainerID, uuid.Nil):
		container = z.objectsById[e.ObjectContainerID]
	}

	obj := NewObject(e.ObjectID, e.Name, e.Keywords, container, e.Capacity, z)
	if container != nil {
		err := container.addObject(obj)
		if err != nil {
			return err
		}
	} else {
		fmt.Printf("WARNING: processing ObjectMigrateInEvent with no resolvable Container\n")
	}

	obj.setContainer(container)
	z.objectsById[obj.ID()] = obj

	return nil
}

func (z *Zone) applyObjectMigrateOutEvent(e *ObjectMigrateOutEvent) error {
	object, found := z.objectsById[e.ObjectID]
	if !found {
		return fmt.Errorf("cannot find Object %q to migrate out of Zone", e.ObjectID)
	}
	object.Container().removeObject(object)
	object.setContainer(nil)
	object.setZone(nil)
	delete(z.objectsById, object.ID())

	return nil
}

func (z *Zone) applyZoneSetDefaultLocationEvent(e *ZoneSetDefaultLocationEvent) error {
	loc, ok := z.locationsById[e.LocationID]
	if !ok {
		return fmt.Errorf("no such Location with ID %q in Zone", e.LocationID)
	}

	z.defaultLocation = loc
	return nil
}

func (z *Zone) snapshot(sequenceNum uint64) []Event {
	setSize := len(z.locationsById) + len(z.exitsById) + len(z.actorsById) + len(z.objectsById)
	visited := make(map[snapshottable]bool, setSize)
	for _, actor := range z.actorsById {
		visited[actor] = false
	}
	for _, loc := range z.locationsById {
		visited[loc] = false
	}
	for _, exit := range z.exitsById {
		visited[exit] = false
	}
	for _, obj := range z.objectsById {
		visited[obj] = false
	}

	orderedObjs := make([]snapshottable, 0, setSize)
	for snappable := range visited {
		orderedObjs = append(orderedObjs, dfsSnapshottableDepsOmittingVisited(snappable, visited)...)
	}

	snapEvents := make([]Event, 0, setSize)
	for _, obj := range orderedObjs {
		snapEvents = append(snapEvents, obj.snapshot(sequenceNum))
	}

	return snapEvents
}

func dfsSnapshottableDepsOmittingVisited(this snapshottable, visitedMap map[snapshottable]bool) []snapshottable {
	if visitedMap[this] {
		return nil
	}
	var ret []snapshottable
	for _, dep := range this.snapshotDependencies() {
		ret = append(ret, dfsSnapshottableDepsOmittingVisited(dep, visitedMap)...)
	}
	ret = append(ret, this)
	visitedMap[this] = true
	return ret
}

func newZoneSetDefaultLocationCommand(wrapped *ZoneSetDefaultLocationEvent) zoneSetDefaultLocationCommand {
	return zoneSetDefaultLocationCommand{
		commandGeneric{commandType: CommandTypeZoneSetDefaultLocation},
		wrapped,
	}
}

type zoneSetDefaultLocationCommand struct {
	commandGeneric
	wrappedEvent *ZoneSetDefaultLocationEvent
}

func NewZoneSetDefaultLocationEvent(locID, zoneID uuid.UUID) *ZoneSetDefaultLocationEvent {
	return &ZoneSetDefaultLocationEvent{
		eventGeneric: &eventGeneric{
			eventType:     EventTypeZoneSetDefaultLocation,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		LocationID: locID,
	}
}

type ZoneSetDefaultLocationEvent struct {
	*eventGeneric
	LocationID uuid.UUID
}

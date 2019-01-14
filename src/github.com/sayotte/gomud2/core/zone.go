package core

import (
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/rpc"
	myuuid "github.com/sayotte/gomud2/uuid"
	"log"
	"sync"
	"time"
)

func NewZone(id uuid.UUID, persister EventPersister) *Zone {
	newID := id
	if uuid.Equal(id, uuid.Nil) {
		newID = myuuid.NewId()
	}
	return &Zone{
		id:            newID,
		actorsById:    make(map[uuid.UUID]*Actor),
		locationsById: make(map[uuid.UUID]*Location),
		edgesById:     make(map[uuid.UUID]*LocationEdge),
		objectsById:   make(map[uuid.UUID]*Object),
		persister:     persister,
	}
}

type Zone struct {
	id             uuid.UUID
	world          *World
	nextSequenceId uint64
	actorsById     map[uuid.UUID]*Actor
	locationsById  map[uuid.UUID]*Location
	edgesById      map[uuid.UUID]*LocationEdge
	objectsById    map[uuid.UUID]*Object
	// This is the channel where the Zone picks up new events submitted by its
	// own public methods. This should never be directly exposed by an accessor;
	// public methods should create requests and send them to the channel.
	internalRequestChan chan rpc.Request
	stopChan            chan struct{}
	stopWG              *sync.WaitGroup
	persister           EventPersister
}

func (z *Zone) ID() uuid.UUID {
	return z.id
}

func (z *Zone) setPersister(ep EventPersister) {
	z.persister = ep
}

func (z *Zone) LastSequenceNum() uint64 {
	return z.nextSequenceId - 1
}

func (z *Zone) syncRequestToSelf(e Event) (interface{}, error) {
	req := rpc.NewRequest(e)
	z.internalRequestChan <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (z *Zone) StartEventProcessing() {
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

			// absorb *one* request from each edge/location/object/actor
			for _, edge := range z.edgesById {
				select {
				case req := <-edge.requestChan:
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
				e := req.Payload.(Event)
				value, err := z.processEvent(e)
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

// this is used to rebuild state from an Event store; it is not to be used
// during normal operations
func (z *Zone) ReplayEvents(inChan <-chan rpc.Response) error {
	for res := range inChan {
		if res.Err != nil {
			return res.Err
		}
		_, err := z.processEvent(res.Value.(Event))
		if err != nil {
			return err
		}
	}
	return nil
}

func (z *Zone) processEvent(e Event) (interface{}, error) {
	if e.SequenceNumber() == 0 {
		e.SetSequenceNumber(z.nextSequenceId)
	}
	log.Printf("DEBUG: zone processing event %d", e.SequenceNumber())
	out, err := z.apply(e)
	if err != nil {
		return nil, err
	}
	if z.persister != nil {
		err = z.persister.PersistEvent(e)
	}
	z.nextSequenceId = e.SequenceNumber() + 1

	return out, err
}

func (z *Zone) StopEventProcessing() {
	if z.stopWG == nil {
		z.stopWG = &sync.WaitGroup{}
	}
	z.stopWG.Add(1)
	close(z.stopChan)
	z.stopWG.Wait()
}

func (z *Zone) ActorByID(id uuid.UUID) *Actor {
	return z.actorsById[id]
}

func (z *Zone) LocationByID(id uuid.UUID) *Location {
	return z.locationsById[id]
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

func (z *Zone) sendEventToObservers(e Event, oList ObserverList) {
	for _, o := range oList {
		o.SendEvent(e)
	}
}

func (z *Zone) apply(e Event) (interface{}, error) {
	switch e.Type() {
	case EventTypeActorAddToZone:
		typedEvent := e.(ActorAddToZoneEvent)
		return z.applyActorAddToZoneEvent(typedEvent)
	case EventTypeActorMove:
		typedEvent := e.(*ActorMoveEvent)
		return nil, z.applyActorMoveEvent(typedEvent)
	case EventTypeActorRemoveFromZone:
		typedEvent := e.(ActorRemoveFromZoneEvent)
		return nil, z.applyActorRemoveEvent(typedEvent)
	case EventTypeLocationAddToZone:
		typedEvent := e.(LocationAddToZoneEvent)
		return z.applyLocationAddToZoneEvent(typedEvent)
	case EventTypeLocationEdgeAddToZone:
		typedEvent := e.(LocationEdgeAddToZoneEvent)
		return z.applyLocationEdgeAddToZoneEvent(typedEvent)
	case EventTypeObjectAddToZone:
		typedEvent := e.(ObjectAddToZoneEvent)
		return z.applyObjectAddToZoneEvent(typedEvent)
	case EventTypeObjectMove:
		typedEvent := e.(ObjectMoveEvent)
		return nil, z.applyObjectMoveEvent(typedEvent)
	default:
		return nil, fmt.Errorf("unknown Event type %T", e)
	}
}

func (z *Zone) applyActorMoveEvent(e *ActorMoveEvent) error {
	fromLoc, ok := z.locationsById[e.fromLocationId]
	if !ok {
		return fmt.Errorf("unknown from-location %q", e.fromLocationId)
	}
	toLoc, ok := z.locationsById[e.toLocationId]
	if !ok {
		return fmt.Errorf("unknown to-location %q", e.toLocationId)
	}
	actor, ok := z.actorsById[e.actorId]
	if !ok {
		return fmt.Errorf("unknown Actor %q", e.actorId)
	}

	err := fromLoc.removeActor(actor)
	if err != nil {
		return err
	}
	err = toLoc.addActor(actor)
	if err != nil {
		return err
	}
	actor.setLocation(toLoc)

	var oList ObserverList
	for _, o := range actor.Observers() {
		oList = append(oList, o)
	}
	for _, o := range fromLoc.Observers() {
		oList = append(oList, o)
	}
	for _, o := range toLoc.Observers() {
		oList = append(oList, o)
	}
	z.sendEventToObservers(e, oList)

	return nil
}

func (z *Zone) AddActor(a *Actor) (*Actor, error) {
	e := a.snapshot(0)
	val, err := z.syncRequestToSelf(e)
	newActor := val.(*Actor)
	return newActor, err
}

// MigrateInActor is a wrapper for AddActor, ensuring the given Observer
// receives the associated AddActorToZone event. Otherwise, since we're
// returning a new Actor object, the Observer would not already be associated
// with that new object and would never see the event.
//
// This function is never called when replaying our event-stream and there is
// no corresponding Event type-- it is purely to ensure correct communication
// as a migration is happening live.
func (z *Zone) MigrateInActor(a *Actor, o Observer) (*Actor, error) {
	a.Location().addObserver(o)
	newActor, err := z.AddActor(a)
	a.Location().removeObserver(o)
	return newActor, err
}

func (z *Zone) applyActorAddToZoneEvent(e ActorAddToZoneEvent) (*Actor, error) {
	newLoc, ok := z.locationsById[e.startingLocationId]
	if !ok {
		return nil, fmt.Errorf("unknown startingLocation %q", e.startingLocationId)
	}
	_, duplicate := z.actorsById[e.actorId]
	if duplicate {
		return nil, fmt.Errorf("Actor %q already present in zone", e.actorId)
	}
	actor := NewActor(e.ActorID(), e.name, newLoc, z)

	err := newLoc.addActor(actor)
	if err != nil {
		return nil, err
	}
	actor.setLocation(newLoc)
	z.actorsById[actor.ID()] = actor

	oList := newLoc.Observers()
	z.sendEventToObservers(e, oList)

	return actor, nil
}

func (z *Zone) RemoveActor(a *Actor) error {
	remEvent := NewActorRemoveFromZoneEvent(a.ID(), z.id)
	_, err := z.syncRequestToSelf(remEvent)
	return err
}

func (z *Zone) applyActorRemoveEvent(e ActorRemoveFromZoneEvent) error {
	actor, found := z.actorsById[e.actorID]
	if !found {
		return fmt.Errorf("Actor %q not found in Zone", e.actorID)
	}
	oldLoc := actor.Location()
	err := oldLoc.removeActor(actor)
	if err != nil {
		return err
	}
	actor.setLocation(nil)
	actor.setZone(nil)
	delete(z.actorsById, e.actorID)

	oList := oldLoc.Observers()
	z.sendEventToObservers(e, oList)

	return nil
}

func (z *Zone) AddLocation(l *Location) (*Location, error) {
	e := l.snapshot(0)
	val, err := z.syncRequestToSelf(e)
	if err != nil {
		return nil, err
	}
	newLoc := val.(*Location)
	return newLoc, nil
}

func (z *Zone) applyLocationAddToZoneEvent(e LocationAddToZoneEvent) (*Location, error) {
	_, duplicate := z.locationsById[e.locationId]
	if duplicate {
		return nil, fmt.Errorf("location with ID %q already present in zone", e.locationId)
	}
	loc := NewLocation(
		e.locationId,
		z,
		e.shortDesc,
		e.desc,
	)
	z.locationsById[e.locationId] = loc
	return loc, nil
}

func (z *Zone) AddLocationEdge(le *LocationEdge) (*LocationEdge, error) {
	e := le.snapshot(0)
	val, err := z.syncRequestToSelf(e)
	if err != nil {
		return nil, err
	}
	newEdge := val.(*LocationEdge)
	return newEdge, nil
}

func (z *Zone) applyLocationEdgeAddToZoneEvent(e LocationEdgeAddToZoneEvent) (*LocationEdge, error) {
	_, duplicate := z.edgesById[e.EdgeId]
	if duplicate {
		return nil, fmt.Errorf("Edge with ID %q already present in zone", e.EdgeId)
	}
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

	edge := NewLocationEdge(
		e.EdgeId,
		e.Description,
		e.Direction,
		srcLoc,
		destLoc,
		z,
		e.DestZoneID,
		destLocID,
	)
	err := srcLoc.addOutEdge(edge)
	if err != nil {
		return nil, err
	}
	z.edgesById[edge.ID()] = edge
	return edge, nil
}

func (z *Zone) AddObject(o *Object, startingLocation *Location) (*Object, error) {
	e := o.snapshot(0)
	val, err := z.syncRequestToSelf(e)
	newObject := val.(*Object)
	return newObject, err
}

func (z *Zone) applyObjectAddToZoneEvent(e ObjectAddToZoneEvent) (*Object, error) {
	newLoc, ok := z.locationsById[e.startingLocationId]
	if !ok {
		return nil, fmt.Errorf("unknown startingLocation %q", e.startingLocationId)
	}
	_, duplicate := z.objectsById[e.objectId]
	if duplicate {
		return nil, fmt.Errorf("Object with ID %q already present in zone", e.objectId)
	}

	obj := NewObject(e.objectId, e.name, newLoc, z)
	err := newLoc.addObject(obj)
	if err != nil {
		return nil, err
	}
	obj.setLocation(newLoc)
	z.objectsById[obj.ID()] = obj

	z.sendEventToObservers(e, newLoc.Observers())

	return obj, nil
}

func (z *Zone) applyObjectMoveEvent(e ObjectMoveEvent) error {
	fromLoc, ok := z.locationsById[e.fromLocationId]
	if !ok {
		return fmt.Errorf("unknown from-location %q", e.fromLocationId)
	}
	toLoc, ok := z.locationsById[e.toLocationId]
	if !ok {
		return fmt.Errorf("unknown to-location %q", e.toLocationId)
	}
	obj, ok := z.objectsById[e.objectId]
	if !ok {
		return fmt.Errorf("unknown Object %q", e.objectId)
	}

	err := fromLoc.removeObject(obj)
	if err != nil {
		return err
	}
	err = toLoc.addObject(obj)
	if err != nil {
		return err
	}
	obj.setLocation(toLoc)

	var oList ObserverList
	for _, o := range fromLoc.Observers() {
		oList = append(oList, o)
	}
	for _, o := range toLoc.Observers() {
		oList = append(oList, o)
	}
	z.sendEventToObservers(e, oList)

	return nil
}

func (z *Zone) snapshot(sequenceNum uint64) []Event {
	setSize := len(z.locationsById) + len(z.edgesById) + len(z.actorsById) + len(z.objectsById)
	visited := make(map[snapshottable]bool, setSize)
	for _, actor := range z.actorsById {
		visited[actor] = false
	}
	for _, loc := range z.locationsById {
		visited[loc] = false
	}
	for _, edge := range z.edgesById {
		visited[edge] = false
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

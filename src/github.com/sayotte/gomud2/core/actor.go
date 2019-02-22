package core

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/rpc"
	myuuid "github.com/sayotte/gomud2/uuid"
)

var actorMoveDelay = time.Millisecond * 500

func NewActor(id uuid.UUID, name, brainType string, location *Location, zone *Zone, attrs AttributeSet, skills Skillset, inventoryConstraints ActorInventoryConstraints) *Actor {
	newID := id
	if uuid.Equal(id, uuid.Nil) {
		newID = myuuid.NewId()
	}
	return &Actor{
		id:         newID,
		name:       name,
		location:   location,
		zone:       zone,
		brainType:  brainType,
		inventory:  &ActorInventory{constraints: inventoryConstraints},
		rwlock:     &sync.RWMutex{},
		attributes: attrs,
		skills:     skills,
	}
}

type Actor struct {
	id                     uuid.UUID
	name                   string
	location               *Location
	zone                   *Zone
	observers              ObserverList
	nextDelayedActionStart time.Time

	brainType string

	inventory *ActorInventory

	rwlock *sync.RWMutex

	attributes AttributeSet
	skills     Skillset
}

//////// getters + non-command-setters

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

func (a *Actor) BrainType() string {
	return a.brainType
}

func (a *Actor) Attributes() AttributeSet {
	return a.attributes
}

func (a *Actor) setAttributes(attrs AttributeSet) {
	a.attributes = attrs
}

func (a *Actor) Skills() Skillset {
	return a.skills
}

func (a *Actor) Inventory() *ActorInventory {
	return a.inventory
}

func (a *Actor) Capacity() int {
	return a.inventory.Capacity()
}

func (a *Actor) Objects() ObjectList {
	return a.inventory.Objects()
}

func (a *Actor) ContainsObject(o *Object) bool {
	return a.inventory.ContainsObject(o)
}

func (a *Actor) SubcontainerFor(o *Object) string {
	return a.inventory.SubcontainerFor(o)
}

func (a *Actor) addObject(o *Object, subcontainer string) error {
	return a.inventory.addObject(o, subcontainer)
}

func (a *Actor) removeObject(o *Object) {
	a.inventory.removeObject(o)
}

func (a *Actor) checkMoveObjectToSubcontainer(o *Object, oldSub, newSub string) error {
	return a.inventory.checkMoveObjectToSubcontainer(o, oldSub, newSub)
}

func (a *Actor) moveObjectToSubcontainer(o *Object, oldSub, newSub string) error {
	return a.inventory.moveObjectToSubcontainer(o, oldSub, newSub)
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

func (a Actor) snapshot(sequenceNum uint64) Event {
	e := NewActorAddToZoneEvent(
		a.name,
		a.brainType,
		a.id,
		a.location.ID(),
		a.zone.ID(),
		a.attributes,
		a.skills,
		a.inventory.Constraints(),
	)
	e.SetSequenceNumber(sequenceNum)
	return e
}

func (a Actor) snapshotDependencies() []snapshottable {
	return []snapshottable{a.location}
}

//////// command methods

func (a *Actor) Move(from, to *Location) error {
	if from.Zone() != to.Zone() {
		return fmt.Errorf("cross-zone moves should use the World.MigrateZone() API call")
	}

	delayTilActionStart := a.nextDelayedActionStart.Sub(time.Now())
	if delayTilActionStart > 0 {
		time.Sleep(delayTilActionStart)
	}

	e := NewActorMoveEvent(
		from.ID(),
		to.ID(),
		a.id,
		a.zone.ID(),
	)
	cmd := newActorMoveCommand(e)
	_, err := a.syncRequestToZone(cmd)

	if err == nil {
		a.nextDelayedActionStart = time.Now().Add(actorMoveDelay)
	}

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

func (a *Actor) Slash(target *Actor) error {
	//if target.Location() != a.location {
	//	return fmt.Errorf("target not in same Location")
	//}
	dmgCmd := newCombatMeleeCommand(a, target, CombatMeleeDamageTypeSlash)
	_, err := a.syncRequestToZone(dmgCmd)
	return err
}

func (a *Actor) Die() error {
	c := NewActorDeathCommand(a)
	_, err := a.syncRequestToZone(c)
	return err
}

func (a *Actor) syncRequestToZone(c Command) (interface{}, error) {
	req := rpc.NewRequest(c)
	a.zone.requestChan() <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

///////////////////////////////// ActorList ///////////////////////////////////

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

///////////////////////////// Commands and Events /////////////////////////////

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
			EventTypeNum:      EventTypeActorMove,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneId,
			ShouldPersistBool: true,
		},
		fromLocationId,
		toLocationId,
		actorId,
	}
}

type ActorMoveEvent struct {
	*eventGeneric
	FromLocationId uuid.UUID
	ToLocationId   uuid.UUID
	ActorId        uuid.UUID
}

func (ame ActorMoveEvent) FromToActorIDs() (uuid.UUID, uuid.UUID, uuid.UUID) {
	return ame.FromLocationId, ame.ToLocationId, ame.ActorId
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
			EventTypeNum:      EventTypeActorAdminRelocate,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
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

func NewActorAddToZoneEvent(name, brainType string, actorId, startingLocationId, zoneId uuid.UUID, attrs AttributeSet, skills Skillset, invConstraints ActorInventoryConstraints) *ActorAddToZoneEvent {
	return &ActorAddToZoneEvent{
		&eventGeneric{
			EventTypeNum:      EventTypeActorAddToZone,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneId,
			ShouldPersistBool: true,
		},
		actorId,
		name,
		brainType,
		startingLocationId,
		attrs,
		skills,
		invConstraints,
	}
}

type ActorAddToZoneEvent struct {
	*eventGeneric
	ActorID              uuid.UUID
	Name, BrainType      string
	StartingLocationID   uuid.UUID
	Attributes           AttributeSet
	Skills               Skillset
	InventoryConstraints ActorInventoryConstraints
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

func NewActorRemoveFromZoneEvent(actorID, zoneID uuid.UUID) *ActorRemoveFromZoneEvent {
	return &ActorRemoveFromZoneEvent{
		&eventGeneric{
			EventTypeNum:      EventTypeActorRemoveFromZone,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
		},
		actorID,
	}
}

type ActorRemoveFromZoneEvent struct {
	*eventGeneric
	ActorID uuid.UUID
}

func NewActorDeathCommand(actor *Actor) *actorDeathCommand {
	return &actorDeathCommand{
		commandGeneric{commandType: CommandTypeActorDeath},
		actor,
	}
}

type actorDeathCommand struct {
	commandGeneric
	actor *Actor
}

func NewActorDeathEvent(name string, actorID, zoneID uuid.UUID) *ActorDeathEvent {
	return &ActorDeathEvent{
		&eventGeneric{
			EventTypeNum:      EventTypeActorDeath,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
		},
		name,
		actorID,
	}
}

type ActorDeathEvent struct {
	*eventGeneric
	ActorName string
	ActorID   uuid.UUID
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

func NewActorMigrateInEvent(name, brainType string, actorID, fromLocID, fromZoneID, toLocID, zoneID uuid.UUID, attrs AttributeSet, skills Skillset, invConstraints ActorInventoryConstraints) *ActorMigrateInEvent {
	return &ActorMigrateInEvent{
		eventGeneric: &eventGeneric{
			EventTypeNum:      EventTypeActorMigrateIn,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
		},
		Name:                 name,
		BrainType:            brainType,
		ActorID:              actorID,
		FromLocID:            fromLocID,
		FromZoneID:           fromZoneID,
		ToLocID:              toLocID,
		Attributes:           attrs,
		Skills:               skills,
		InventoryConstraints: invConstraints,
	}
}

type ActorMigrateInEvent struct {
	*eventGeneric
	ActorID               uuid.UUID
	Name, BrainType       string
	FromLocID, FromZoneID uuid.UUID
	ToLocID               uuid.UUID
	Attributes            AttributeSet
	Skills                Skillset
	InventoryConstraints  ActorInventoryConstraints
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
			EventTypeNum:      EventTypeActorMigrateOut,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
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

/////////////////// Reusable code closely related to Actors ///////////////////

func doActorDeath(actor *Actor, zone *Zone) []Event {
	var outEvents []Event

	// Create the death event itself
	deathEv := NewActorDeathEvent(actor.Name(), actor.ID(), zone.id)
	outEvents = append(outEvents, deathEv)

	// Create a corpse to hold the objects previously held by the Actor
	corpseObjectProto := NewObject(
		uuid.Nil,
		actor.Name()+"'s corpse",
		"The empty husk of what was once a living thing.",
		[]string{"corpse"},
		actor.Location(),
		100,
		zone,
		ObjectAttributes{},
	)
	corpseObjEv := corpseObjectProto.snapshot(zone.nextSequenceId)
	outEvents = append(outEvents, corpseObjEv)

	// Relocate all the Actor's objects to the corpse
	for _, objContTuple := range getObjectContainerTuplesRecursive(actor) {
		objEv := NewObjectAdminRelocateEvent(objContTuple.obj.ID(), zone.id)
		objEv.ToObjectContainerID = corpseObjectProto.ID()
		outEvents = append(outEvents, objEv)
	}

	// Remove the Actor, as it's supposed to be dead and has been replaced by
	// a corpse
	remActorEv := NewActorRemoveFromZoneEvent(actor.ID(), zone.id)
	outEvents = append(outEvents, remActorEv)

	return outEvents
}

package core

import (
	"fmt"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/rpc"
	myuuid "github.com/sayotte/gomud2/uuid"
)

func NewLocation(id uuid.UUID, zone *Zone, shortDesc, longDesc string) *Location {
	newID := id
	if uuid.Equal(id, uuid.Nil) {
		newID = myuuid.NewId()
	}
	return &Location{
		id:               newID,
		zone:             zone,
		shortDescription: shortDesc,
		description:      longDesc,
		requestChan:      make(chan rpc.Request),
	}
}

type Location struct {
	id               uuid.UUID
	zone             *Zone
	shortDescription string // e.g. "a house in the woods"
	description      string // e.g. "A quaint house with blue shutters .... etc."
	actors           ActorList
	objects          ObjectList
	outExits         ExitList
	observers        ObserverList
	// This is the channel where the Zone picks up new events related to this
	// Location. This should never be directly exposed by an accessor; public methods
	// should create events and send them to the channel.
	requestChan chan rpc.Request
}

func (l Location) ID() uuid.UUID {
	return l.id
}

func (l Location) Tag() string {
	return fmt.Sprintf("%s/%s", l.shortDescription, l.id)
}

func (l Location) Zone() *Zone {
	return l.zone
}

func (l Location) ShortDescription() string {
	return l.shortDescription
}

func (l *Location) setShortDescription(s string) {
	l.shortDescription = s
}

func (l Location) Description() string {
	return l.description
}

func (l *Location) setDescription(s string) {
	l.description = s
}

func (l Location) Observers() ObserverList {
	oList := make(ObserverList, 0, len(l.actors)+len(l.observers))
	for _, actor := range l.actors {
		oList = append(oList, actor.Observers()...)
	}
	oList = append(oList, l.observers...)
	return oList
}

func (l *Location) addObserver(o Observer) {
	l.observers = append(l.observers, o)
}

func (l *Location) removeObserver(o Observer) {
	l.observers = l.observers.Remove(o)
}

func (l Location) Actors() ActorList {
	return l.actors.Copy()
}

func (l *Location) removeActor(actor *Actor) error {
	idx, err := l.actors.IndexOf(actor)
	if err != nil {
		return fmt.Errorf("cannot remove Actor %q from location %q", actor.ID(), l.id)
	}
	l.actors = append(l.actors[:idx], l.actors[idx+1:]...)
	return nil
}

func (l *Location) addActor(actor *Actor) error {
	_, err := l.actors.IndexOf(actor)
	if err == nil {
		return fmt.Errorf("Actor %q already present at location %q", actor.ID(), l.id)
	}
	l.actors = append(l.actors, actor)
	return nil
}

func (l Location) Objects() ObjectList {
	return l.objects.Copy()
}

func (l *Location) removeObject(object *Object) error {
	idx, err := l.objects.IndexOf(object)
	if err != nil {
		return fmt.Errorf("cannot remove Object %q from location %q", object.ID(), l.id)
	}
	l.objects = append(l.objects[:idx], l.objects[idx+1:]...)
	return nil
}

func (l *Location) addObject(object *Object) error {
	_, err := l.objects.IndexOf(object)
	if err == nil {
		return fmt.Errorf("Object %q already present at location %q", object.ID(), l.id)
	}
	l.objects = append(l.objects, object)
	return nil
}

func (l Location) OutExits() ExitList {
	return l.outExits.Copy()
}

func (l *Location) removeExit(exit *Exit) error {
	idx, err := l.outExits.IndexOf(exit)
	if err != nil {
		return fmt.Errorf("cannot remove Exit %q from Location %q: %s", exit.ID(), l.id, err)
	}
	l.outExits = append(l.outExits[:idx], l.outExits[idx+1:]...)
	return nil
}

func (l *Location) addOutExit(exit *Exit) error {
	_, err := l.outExits.IndexOf(exit)
	if err == nil {
		return fmt.Errorf("Exit %q already present at Location %q", exit.ID(), l.id)
	}
	for _, existingExit := range l.outExits {
		if existingExit.Direction() == exit.Direction() {
			return fmt.Errorf("out-Exit from location %q in direction %s already present", l.id, exit.Direction())
		}
	}
	l.outExits = append(l.outExits, exit)
	return nil
}

func (l *Location) setZone(z *Zone) {
	l.zone = z
}

func (l Location) Update(shortDesc, desc string) error {
	e := NewLocationUpdateEvent(
		shortDesc,
		desc,
		l.id,
		l.zone.ID(),
	)
	_, err := l.syncRequestToZone(e)
	return err
}

func (l *Location) syncRequestToZone(e Event) (interface{}, error) {
	req := rpc.NewRequest(e)
	l.requestChan <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (l Location) snapshot(sequenceNum uint64) Event {
	e := NewLocationAddToZoneEvent(
		l.shortDescription,
		l.description,
		l.id,
		l.zone.ID(),
	)
	e.SetSequenceNumber(sequenceNum)
	return e
}

func (l Location) snapshotDependencies() []snapshottable {
	return nil
}

type LocationList []*Location

func (ll LocationList) IndexOf(loc *Location) (int, error) {
	for i := 0; i < len(ll); i++ {
		if ll[i] == loc {
			return i, nil
		}
	}
	return -1, fmt.Errorf("location %q not found in list", loc.id)
}

func newLocationAddToZoneCommand(wrapped LocationAddToZoneEvent) locationAddToZoneCommand {
	return locationAddToZoneCommand{
		commandGeneric{commandType: CommandTypeLocationAddToZone},
		wrapped,
	}
}

type locationAddToZoneCommand struct {
	commandGeneric
	wrappedEvent LocationAddToZoneEvent
}

func NewLocationAddToZoneEvent(shortDesc, desc string, locationId, zoneId uuid.UUID) LocationAddToZoneEvent {
	return LocationAddToZoneEvent{
		&eventGeneric{
			eventType:     EventTypeLocationAddToZone,
			version:       1,
			aggregateId:   zoneId,
			shouldPersist: true,
		},
		locationId,
		shortDesc,
		desc,
	}
}

type LocationAddToZoneEvent struct {
	*eventGeneric
	locationId uuid.UUID
	shortDesc  string
	desc       string
}

func (latze LocationAddToZoneEvent) LocationID() uuid.UUID {
	return latze.locationId
}

func (latze LocationAddToZoneEvent) ShortDescription() string {
	return latze.shortDesc
}

func (latze LocationAddToZoneEvent) Description() string {
	return latze.desc
}

func newLocationUpdateCommand(wrapped LocationUpdateEvent) locationUpdateCommand {
	return locationUpdateCommand{
		commandGeneric{commandType: CommandTypeLocationUpdate},
		wrapped,
	}
}

type locationUpdateCommand struct {
	commandGeneric
	wrappedEvent LocationUpdateEvent
}

func NewLocationUpdateEvent(shortDesc, desc string, locationID, zoneID uuid.UUID) LocationUpdateEvent {
	return LocationUpdateEvent{
		&eventGeneric{
			eventType:     EventTypeLocationUpdate,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		locationID,
		shortDesc,
		desc,
	}
}

type LocationUpdateEvent struct {
	*eventGeneric
	locationID uuid.UUID
	shortDesc  string
	desc       string
}

func (lue LocationUpdateEvent) LocationID() uuid.UUID {
	return lue.locationID
}

func (lue LocationUpdateEvent) ShortDescription() string {
	return lue.shortDesc
}

func (lue LocationUpdateEvent) Description() string {
	return lue.desc
}

func newLocationRemoveFromZoneCommand(wrapped LocationRemoveFromZoneEvent) locationRemoveFromZoneCommand {
	return locationRemoveFromZoneCommand{
		commandGeneric{commandType: CommandTypeLocationRemoveFromZone},
		wrapped,
	}
}

type locationRemoveFromZoneCommand struct {
	commandGeneric
	wrappedEvent LocationRemoveFromZoneEvent
}

func NewLocationRemoveFromZoneEvent(locID, zoneID uuid.UUID) LocationRemoveFromZoneEvent {
	return LocationRemoveFromZoneEvent{
		eventGeneric: &eventGeneric{
			eventType:     EventTypeLocationRemoveFromZone,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		LocationID: locID,
	}
}

type LocationRemoveFromZoneEvent struct {
	*eventGeneric
	LocationID uuid.UUID
}

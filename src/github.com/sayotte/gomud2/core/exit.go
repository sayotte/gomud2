package core

import (
	"fmt"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/rpc"
	myuuid "github.com/sayotte/gomud2/uuid"
)

const (
	ExitDirectionNorth = "north"
	ExitDirectionSouth = "south"
	ExitDirectionEast  = "east"
	ExitDirectionWest  = "west"
)

var ValidDirections = map[string]bool{
	ExitDirectionNorth: true,
	ExitDirectionSouth: true,
	ExitDirectionEast:  true,
	ExitDirectionWest:  true,
}

func NewExit(id uuid.UUID, desc, direction string, src, dest *Location, zone *Zone, otherZoneID, otherLocID uuid.UUID) *Exit {
	newID := id
	if uuid.Equal(id, uuid.Nil) {
		newID = myuuid.NewId()
	}
	return &Exit{
		id:             newID,
		description:    desc,
		direction:      direction,
		source:         src,
		destination:    dest,
		zone:           zone,
		otherZoneID:    otherZoneID,
		otherZoneLocID: otherLocID,
		requestChan:    make(chan rpc.Request),
	}
}

type Exit struct {
	id             uuid.UUID
	description    string // e.g. "a small door"
	direction      string // e.g. "north" or "forward"
	source         *Location
	destination    *Location
	zone           *Zone
	otherZoneID    uuid.UUID
	otherZoneLocID uuid.UUID
	// This is the channel where the Zone picks up new events related to this
	// Exit. This should never be directly exposed by an accessor; public methods
	// should create events and send them to the channel.
	requestChan chan rpc.Request
}

func (ex Exit) ID() uuid.UUID {
	return ex.id
}

func (ex Exit) Description() string {
	return ex.description
}

func (ex *Exit) setDescription(s string) {
	ex.description = s
}

func (ex Exit) Direction() string {
	return ex.direction
}

func (ex *Exit) setDirection(s string) {
	ex.direction = s
}

func (ex Exit) Source() *Location {
	return ex.source
}

func (ex *Exit) setSource(loc *Location) {
	ex.source = loc
}

func (ex Exit) Destination() *Location {
	return ex.destination
}

func (ex *Exit) setDestination(loc *Location) {
	ex.destination = loc
}

func (ex Exit) OtherZoneID() uuid.UUID {
	return ex.otherZoneID
}

func (ex *Exit) setOtherZoneID(id uuid.UUID) {
	ex.otherZoneID = id
}

func (ex Exit) OtherZoneLocID() uuid.UUID {
	return ex.otherZoneLocID
}

func (ex *Exit) setOtherZoneLocID(id uuid.UUID) {
	ex.otherZoneLocID = id
}

func (ex Exit) Zone() *Zone {
	return ex.zone
}

func (ex Exit) Update(desc, direction string, source, dest *Location, extZoneID, extLocID uuid.UUID) error {
	var destID uuid.UUID
	if dest != nil {
		destID = dest.ID()
	} else {
		destID = extLocID
	}
	e := NewExitUpdateEvent(
		desc,
		direction,
		ex.ID(),
		source.ID(),
		destID,
		ex.zone.ID(),
		extZoneID,
	)
	_, err := ex.syncRequestToZone(e)
	return err
}

func (ex Exit) syncRequestToZone(e Event) (interface{}, error) {
	req := rpc.NewRequest(e)
	ex.requestChan <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (ex Exit) snapshot(sequenceNum uint64) Event {
	var destID uuid.UUID
	if ex.destination != nil {
		destID = ex.destination.ID()
	} else {
		destID = ex.otherZoneLocID
	}
	e := NewExitAddToZoneEvent(
		ex.description,
		ex.direction,
		ex.id,
		ex.source.ID(),
		destID,
		ex.zone.ID(),
		ex.otherZoneID,
	)
	e.SetSequenceNumber(sequenceNum)
	return e
}

func (ex Exit) snapshotDependencies() []snapshottable {
	deps := []snapshottable{ex.source}
	if ex.destination != nil {
		deps = append(deps, ex.destination)
	}
	return deps
}

type ExitList []*Exit

func (el ExitList) IndexOf(exit *Exit) (int, error) {
	for i := 0; i < len(el); i++ {
		if el[i] == exit {
			return i, nil
		}
	}
	return -1, fmt.Errorf("Exit %q not found in list", exit.id)
}

func (el ExitList) Copy() ExitList {
	out := make(ExitList, len(el))
	copy(out, el)
	return out
}

func NewExitAddToZoneEvent(desc, direction string, exitId, sourceId, destLocId, srcZoneId, destZoneID uuid.UUID) ExitAddToZoneEvent {
	return ExitAddToZoneEvent{
		&eventGeneric{
			eventType:     EventTypeExitAddToZone,
			version:       1,
			aggregateId:   srcZoneId,
			shouldPersist: true,
		},
		exitId,
		desc,
		direction,
		sourceId,
		destZoneID,
		destLocId,
	}
}

type ExitAddToZoneEvent struct {
	*eventGeneric
	ExitID           uuid.UUID
	Description      string
	Direction        string
	SourceLocationId uuid.UUID
	DestZoneID       uuid.UUID
	DestLocationId   uuid.UUID
}

func NewExitUpdateEvent(desc, direction string, exitID, sourceID, destID, srcZoneID, extZoneID uuid.UUID) ExitUpdateEvent {
	xue := ExitUpdateEvent{
		eventGeneric: &eventGeneric{
			eventType:     EventTypeExitUpdate,
			version:       1,
			aggregateId:   srcZoneID,
			shouldPersist: true,
		},
		ExitID:           exitID,
		Description:      desc,
		Direction:        direction,
		SourceLocationId: sourceID,
		DestLocationId:   destID,
		DestZoneID:       extZoneID,
	}
	return xue
}

type ExitUpdateEvent struct {
	*eventGeneric
	ExitID           uuid.UUID
	Description      string
	Direction        string
	SourceLocationId uuid.UUID
	DestZoneID       uuid.UUID
	DestLocationId   uuid.UUID
}

func NewExitRemoveFromZoneEvent(exitID, zoneID uuid.UUID) ExitRemoveFromZoneEvent {
	return ExitRemoveFromZoneEvent{
		eventGeneric: &eventGeneric{
			eventType:     EventTypeExitRemoveFromZone,
			version:       1,
			aggregateId:   zoneID,
			shouldPersist: true,
		},
		ExitID: exitID,
	}
}

type ExitRemoveFromZoneEvent struct {
	*eventGeneric
	ExitID uuid.UUID
}

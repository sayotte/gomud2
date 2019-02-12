package core

import (
	"fmt"
	"time"

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
	cmd := newExitUpdateCommand(e)
	_, err := ex.syncRequestToZone(cmd)
	return err
}

func (ex Exit) syncRequestToZone(c Command) (interface{}, error) {
	req := rpc.NewRequest(c)
	ex.zone.requestChan() <- req
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

func (el ExitList) Remove(ex *Exit) ExitList {
	idx, err := el.IndexOf(ex)
	if err != nil {
		return el
	}
	return append(el[:idx], el[idx+1:]...)
}

func newExitAddToZoneCommand(wrapped *ExitAddToZoneEvent) exitAddToZoneCommand {
	return exitAddToZoneCommand{
		commandGeneric{commandType: CommandTypeExitAddToZone},
		wrapped,
	}
}

type exitAddToZoneCommand struct {
	commandGeneric
	wrappedEvent *ExitAddToZoneEvent
}

func NewExitAddToZoneEvent(desc, direction string, exitId, sourceId, destLocId, srcZoneId, destZoneID uuid.UUID) *ExitAddToZoneEvent {
	return &ExitAddToZoneEvent{
		&eventGeneric{
			EventTypeNum:      EventTypeExitAddToZone,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       srcZoneId,
			ShouldPersistBool: true,
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

func newExitUpdateCommand(wrapped *ExitUpdateEvent) exitUpdateCommand {
	return exitUpdateCommand{
		commandGeneric{commandType: CommandTypeExitUpdate},
		wrapped,
	}
}

type exitUpdateCommand struct {
	commandGeneric
	wrappedEvent *ExitUpdateEvent
}

func NewExitUpdateEvent(desc, direction string, exitID, sourceID, destID, srcZoneID, extZoneID uuid.UUID) *ExitUpdateEvent {
	xue := &ExitUpdateEvent{
		eventGeneric: &eventGeneric{
			EventTypeNum:      EventTypeExitUpdate,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       srcZoneID,
			ShouldPersistBool: true,
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

func newExitRemoveFromZoneCommand(wrapped *ExitRemoveFromZoneEvent) exitRemoveFromZoneCommand {
	return exitRemoveFromZoneCommand{
		commandGeneric{commandType: CommandTypeExitRemoveFromZone},
		wrapped,
	}
}

type exitRemoveFromZoneCommand struct {
	commandGeneric
	wrappedEvent *ExitRemoveFromZoneEvent
}

func NewExitRemoveFromZoneEvent(exitID, zoneID uuid.UUID) *ExitRemoveFromZoneEvent {
	return &ExitRemoveFromZoneEvent{
		eventGeneric: &eventGeneric{
			EventTypeNum:      EventTypeExitRemoveFromZone,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
		},
		ExitID: exitID,
	}
}

type ExitRemoveFromZoneEvent struct {
	*eventGeneric
	ExitID uuid.UUID
}

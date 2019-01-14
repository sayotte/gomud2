package core

import (
	"fmt"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/rpc"
	myuuid "github.com/sayotte/gomud2/uuid"
)

const (
	EdgeDirectionNorth = "north"
	EdgeDirectionSouth = "south"
	EdgeDirectionEast  = "east"
	EdgeDirectionWest  = "west"
)

func NewLocationEdge(id uuid.UUID, desc, direction string, src, dest *Location, zone *Zone, otherZoneID, otherLocID uuid.UUID) *LocationEdge {
	newID := id
	if uuid.Equal(id, uuid.Nil) {
		newID = myuuid.NewId()
	}
	return &LocationEdge{
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

type LocationEdge struct {
	id             uuid.UUID
	description    string // e.g. "a small door"
	direction      string // e.g. "north" or "forward"
	source         *Location
	destination    *Location
	zone           *Zone
	otherZoneID    uuid.UUID
	otherZoneLocID uuid.UUID
	// This is the channel where the Zone picks up new events related to this
	// Edge. This should never be directly exposed by an accessor; public methods
	// should create events and send them to the channel.
	requestChan chan rpc.Request
}

func (le LocationEdge) ID() uuid.UUID {
	return le.id
}

func (le LocationEdge) Description() string {
	return le.description
}

func (le LocationEdge) Direction() string {
	return le.direction
}

func (le LocationEdge) Destination() *Location {
	return le.destination
}

func (le LocationEdge) OtherZoneID() uuid.UUID {
	return le.otherZoneID
}

func (le LocationEdge) OtherZoneLocID() uuid.UUID {
	return le.otherZoneLocID
}

func (le LocationEdge) syncRequestToZone(e Event) (interface{}, error) {
	req := rpc.NewRequest(e)
	le.requestChan <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (le LocationEdge) snapshot(sequenceNum uint64) Event {
	var destID uuid.UUID
	if le.destination != nil {
		destID = le.destination.ID()
	} else {
		destID = le.otherZoneLocID
	}
	e := NewLocationEdgeAddToZoneEvent(
		le.description,
		le.direction,
		le.id,
		le.source.ID(),
		destID,
		le.zone.ID(),
		le.otherZoneID,
	)
	e.SetSequenceNumber(sequenceNum)
	return e
}

func (le LocationEdge) snapshotDependencies() []snapshottable {
	deps := []snapshottable{le.source}
	if le.destination != nil {
		deps = append(deps, le.destination)
	}
	return deps
}

type LocationEdgeList []*LocationEdge

func (lel LocationEdgeList) IndexOf(edge *LocationEdge) (int, error) {
	for i := 0; i < len(lel); i++ {
		if lel[i] == edge {
			return i, nil
		}
	}
	return -1, fmt.Errorf("Edge %q not found in list", edge.id)
}

func (lel LocationEdgeList) Copy() LocationEdgeList {
	out := make(LocationEdgeList, len(lel))
	copy(out, lel)
	return out
}

func NewLocationEdgeAddToZoneEvent(desc, direction string, edgeId, sourceId, destLocId, srcZoneId, destZoneID uuid.UUID) LocationEdgeAddToZoneEvent {
	return LocationEdgeAddToZoneEvent{
		&eventGeneric{
			eventType:     EventTypeLocationEdgeAddToZone,
			version:       1,
			aggregateId:   srcZoneId,
			shouldPersist: true,
		},
		edgeId,
		desc,
		direction,
		sourceId,
		destZoneID,
		destLocId,
	}
}

type LocationEdgeAddToZoneEvent struct {
	*eventGeneric
	EdgeId           uuid.UUID
	Description      string
	Direction        string
	SourceLocationId uuid.UUID
	DestZoneID       uuid.UUID
	DestLocationId   uuid.UUID
}

package core

import (
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/rpc"
)

const (
	EdgeDirectionNorth = "north"
	EdgeDirectionSouth = "south"
	EdgeDirectionEast  = "east"
	EdgeDirectionWest  = "west"
)

func NewLocationEdge(id uuid.UUID, desc, direction string, src, dest *Location, zone *Zone, otherZoneID, otherLocID uuid.UUID) *LocationEdge {
	return &LocationEdge{
		Id:             id,
		Description:    desc,
		Direction:      direction,
		Source:         src,
		Destination:    dest,
		Zone:           zone,
		OtherZoneID:    otherZoneID,
		OtherZoneLocID: otherLocID,
	}
}

type LocationEdge struct {
	Id             uuid.UUID
	Description    string // e.g. "a small door"
	Direction      string // should become a new type, but e.g. "north" or "forward"
	Source         *Location
	Destination    *Location
	Zone           *Zone
	OtherZoneID    uuid.UUID
	OtherZoneLocID uuid.UUID
	// This is the channel where the Zone picks up new events related to this
	// Edge. This should never be directly exposed by an accessor; public methods
	// should create events and send them to the channel.
	requestChan chan rpc.Request
}

func (le LocationEdge) syncRequestToZone(e Event) (interface{}, error) {
	req := rpc.NewRequest(e)
	le.requestChan <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (le LocationEdge) snapshot(sequenceNum uint64) Event {
	var destID uuid.UUID
	if le.Destination != nil {
		destID = le.Destination.ID()
	} else {
		destID = le.OtherZoneLocID
	}
	e := NewLocationEdgeAddToZoneEvent(
		le.Description,
		le.Direction,
		le.Id,
		le.Source.ID(),
		destID,
		le.Zone.ID(),
		le.OtherZoneID,
	)
	e.SetSequenceNumber(sequenceNum)
	return e
}

func (le LocationEdge) snapshotDependencies() []snapshottable {
	deps := []snapshottable{le.Source}
	if le.Destination != nil {
		deps = append(deps, le.Destination)
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
	return -1, fmt.Errorf("Edge %q not found in list", edge.Id)
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

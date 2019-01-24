package store

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
)

type locationEdgeAddToZoneEvent struct {
	header                                               eventHeader
	Description, Direction                               string
	EdgeID, SourceLocationID, DestLocationID, DestZoneID uuid.UUID
}

func (leatze locationEdgeAddToZoneEvent) ToDomain() core.Event {
	e := core.NewLocationEdgeAddToZoneEvent(
		leatze.Description,
		leatze.Direction,
		leatze.EdgeID,
		leatze.SourceLocationID,
		leatze.DestLocationID,
		leatze.header.AggregateId,
		leatze.DestZoneID,
	)
	e.SetSequenceNumber(leatze.header.SequenceNumber)
	return e
}

func (leatze *locationEdgeAddToZoneEvent) FromDomain(e core.Event) {
	from := e.(core.LocationEdgeAddToZoneEvent)
	*leatze = locationEdgeAddToZoneEvent{
		header:           eventHeaderFromDomainEvent(from),
		Description:      from.Description,
		Direction:        from.Direction,
		EdgeID:           from.EdgeId,
		SourceLocationID: from.SourceLocationId,
		DestLocationID:   from.DestLocationId,
		DestZoneID:       from.DestZoneID,
	}
}

func (leatze locationEdgeAddToZoneEvent) Header() eventHeader {
	return leatze.header
}

func (leatze *locationEdgeAddToZoneEvent) SetHeader(h eventHeader) {
	leatze.header = h
}

type locationEdgeUpdateEvent struct {
	header                                               eventHeader
	Description, Direction                               string
	EdgeID, SourceLocationID, DestLocationID, DestZoneID uuid.UUID
}

func (leue locationEdgeUpdateEvent) ToDomain() core.Event {
	e := core.NewLocationEdgeUpdateEvent(
		leue.Description,
		leue.Direction,
		leue.EdgeID,
		leue.SourceLocationID,
		leue.DestLocationID,
		leue.header.AggregateId,
		leue.DestZoneID,
	)
	e.SetSequenceNumber(leue.header.SequenceNumber)
	return e
}

func (leue *locationEdgeUpdateEvent) FromDomain(e core.Event) {
	from := e.(core.LocationEdgeUpdateEvent)
	*leue = locationEdgeUpdateEvent{
		header:           eventHeaderFromDomainEvent(from),
		Description:      from.Description,
		Direction:        from.Direction,
		EdgeID:           from.EdgeId,
		SourceLocationID: from.SourceLocationId,
		DestLocationID:   from.DestLocationId,
		DestZoneID:       from.DestZoneID,
	}
}

func (leue locationEdgeUpdateEvent) Header() eventHeader {
	return leue.header
}

func (leue *locationEdgeUpdateEvent) SetHeader(h eventHeader) {
	leue.header = h
}

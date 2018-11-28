package store

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/domain"
)

type locationEdgeAddToZoneEvent struct {
	header                                               eventHeader
	Description, Direction                               string
	EdgeID, SourceLocationID, DestLocationID, DestZoneID uuid.UUID
}

func (leatze locationEdgeAddToZoneEvent) ToDomain() domain.Event {
	e := domain.NewLocationEdgeAddToZoneEvent(
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

func (leatze *locationEdgeAddToZoneEvent) FromDomain(e domain.Event) {
	from := e.(domain.LocationEdgeAddToZoneEvent)
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

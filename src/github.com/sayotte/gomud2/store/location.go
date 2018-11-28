package store

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/domain"
)

type locationAddToZoneEvent struct {
	header          eventHeader
	LocationID      uuid.UUID
	ShortDesc, Desc string
}

func (latze *locationAddToZoneEvent) FromDomain(e domain.Event) {
	from := e.(domain.LocationAddToZoneEvent)
	*latze = locationAddToZoneEvent{
		header:     eventHeaderFromDomainEvent(from),
		LocationID: from.LocationID(),
		ShortDesc:  from.ShortDescription(),
		Desc:       from.Description(),
	}
}

func (latze locationAddToZoneEvent) ToDomain() domain.Event {
	e := domain.NewLocationAddToZoneEvent(
		latze.ShortDesc,
		latze.Desc,
		latze.LocationID,
		latze.header.AggregateId,
	)
	e.SetSequenceNumber(latze.header.SequenceNumber)
	return e
}

func (latze locationAddToZoneEvent) Header() eventHeader {
	return latze.header
}

func (latze *locationAddToZoneEvent) SetHeader(h eventHeader) {
	latze.header = h
}

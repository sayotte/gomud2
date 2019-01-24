package store

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
)

type locationAddToZoneEvent struct {
	header          eventHeader
	LocationID      uuid.UUID
	ShortDesc, Desc string
}

func (latze *locationAddToZoneEvent) FromDomain(e core.Event) {
	from := e.(core.LocationAddToZoneEvent)
	*latze = locationAddToZoneEvent{
		header:     eventHeaderFromDomainEvent(from),
		LocationID: from.LocationID(),
		ShortDesc:  from.ShortDescription(),
		Desc:       from.Description(),
	}
}

func (latze locationAddToZoneEvent) ToDomain() core.Event {
	e := core.NewLocationAddToZoneEvent(
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

type locationUpdateEvent struct {
	header          eventHeader
	LocationID      uuid.UUID
	ShortDesc, Desc string
}

func (lue *locationUpdateEvent) FromDomain(e core.Event) {
	from := e.(core.LocationUpdateEvent)
	*lue = locationUpdateEvent{
		header:     eventHeaderFromDomainEvent(from),
		LocationID: from.LocationID(),
		ShortDesc:  from.ShortDescription(),
		Desc:       from.Description(),
	}
}

func (lue locationUpdateEvent) ToDomain() core.Event {
	e := core.NewLocationUpdateEvent(
		lue.ShortDesc,
		lue.Desc,
		lue.LocationID,
		lue.header.AggregateId,
	)
	e.SetSequenceNumber(lue.header.SequenceNumber)
	return e
}

func (lue locationUpdateEvent) Header() eventHeader {
	return lue.header
}

func (lue *locationUpdateEvent) SetHeader(h eventHeader) {
	lue.header = h
}

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
	from := e.(*core.LocationAddToZoneEvent)
	*latze = locationAddToZoneEvent{
		header:     eventHeaderFromDomainEvent(from),
		LocationID: from.LocationID,
		ShortDesc:  from.ShortDesc,
		Desc:       from.Desc,
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

type locationRemoveFromZoneEvent struct {
	header     eventHeader
	LocationID uuid.UUID
}

func (lrfze *locationRemoveFromZoneEvent) FromDomain(e core.Event) {
	from := e.(*core.LocationRemoveFromZoneEvent)
	*lrfze = locationRemoveFromZoneEvent{
		header:     eventHeaderFromDomainEvent(from),
		LocationID: from.LocationID,
	}
}

func (lrfze locationRemoveFromZoneEvent) ToDomain() core.Event {
	e := core.NewLocationRemoveFromZoneEvent(lrfze.LocationID, lrfze.header.AggregateId)
	e.SetSequenceNumber(lrfze.header.SequenceNumber)
	return e
}

func (lrfze locationRemoveFromZoneEvent) Header() eventHeader {
	return lrfze.header
}

func (lrfze *locationRemoveFromZoneEvent) SetHeader(h eventHeader) {
	lrfze.header = h
}

type locationUpdateEvent struct {
	header          eventHeader
	LocationID      uuid.UUID
	ShortDesc, Desc string
}

func (lue *locationUpdateEvent) FromDomain(e core.Event) {
	from := e.(*core.LocationUpdateEvent)
	*lue = locationUpdateEvent{
		header:     eventHeaderFromDomainEvent(from),
		LocationID: from.LocationID,
		ShortDesc:  from.ShortDesc,
		Desc:       from.Desc,
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

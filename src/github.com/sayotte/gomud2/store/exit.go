package store

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
)

type exitAddToZoneEvent struct {
	header                                               eventHeader
	Description, Direction                               string
	ExitID, SourceLocationID, DestLocationID, DestZoneID uuid.UUID
}

func (eatze exitAddToZoneEvent) ToDomain() core.Event {
	e := core.NewExitAddToZoneEvent(
		eatze.Description,
		eatze.Direction,
		eatze.ExitID,
		eatze.SourceLocationID,
		eatze.DestLocationID,
		eatze.header.AggregateId,
		eatze.DestZoneID,
	)
	e.SetSequenceNumber(eatze.header.SequenceNumber)
	e.SetTimestamp(eatze.header.Timestamp)
	return e
}

func (eatze *exitAddToZoneEvent) FromDomain(e core.Event) {
	from := e.(*core.ExitAddToZoneEvent)
	*eatze = exitAddToZoneEvent{
		header:           eventHeaderFromDomainEvent(from),
		Description:      from.Description,
		Direction:        from.Direction,
		ExitID:           from.ExitID,
		SourceLocationID: from.SourceLocationId,
		DestLocationID:   from.DestLocationId,
		DestZoneID:       from.DestZoneID,
	}
}

func (eatze exitAddToZoneEvent) Header() eventHeader {
	return eatze.header
}

func (eatze *exitAddToZoneEvent) SetHeader(h eventHeader) {
	eatze.header = h
}

type exitUpdateEvent struct {
	header                                               eventHeader
	Description, Direction                               string
	ExitID, SourceLocationID, DestLocationID, DestZoneID uuid.UUID
}

func (exue exitUpdateEvent) ToDomain() core.Event {
	e := core.NewExitUpdateEvent(
		exue.Description,
		exue.Direction,
		exue.ExitID,
		exue.SourceLocationID,
		exue.DestLocationID,
		exue.header.AggregateId,
		exue.DestZoneID,
	)
	e.SetSequenceNumber(exue.header.SequenceNumber)
	e.SetTimestamp(exue.header.Timestamp)
	return e
}

func (exue *exitUpdateEvent) FromDomain(e core.Event) {
	from := e.(*core.ExitUpdateEvent)
	*exue = exitUpdateEvent{
		header:           eventHeaderFromDomainEvent(from),
		Description:      from.Description,
		Direction:        from.Direction,
		ExitID:           from.ExitID,
		SourceLocationID: from.SourceLocationId,
		DestLocationID:   from.DestLocationId,
		DestZoneID:       from.DestZoneID,
	}
}

func (exue exitUpdateEvent) Header() eventHeader {
	return exue.header
}

func (exue *exitUpdateEvent) SetHeader(h eventHeader) {
	exue.header = h
}

type exitRemoveFromZoneEvent struct {
	header eventHeader
	ExitID uuid.UUID
}

func (erfz exitRemoveFromZoneEvent) ToDomain() core.Event {
	e := core.NewExitRemoveFromZoneEvent(erfz.ExitID, erfz.header.AggregateId)
	e.SetSequenceNumber(erfz.header.SequenceNumber)
	e.SetTimestamp(erfz.header.Timestamp)
	return e
}

func (erfz *exitRemoveFromZoneEvent) FromDomain(e core.Event) {
	from := e.(*core.ExitRemoveFromZoneEvent)
	*erfz = exitRemoveFromZoneEvent{
		header: eventHeaderFromDomainEvent(from),
		ExitID: from.ExitID,
	}
}

func (erfz exitRemoveFromZoneEvent) Header() eventHeader {
	return erfz.header
}

func (erfz *exitRemoveFromZoneEvent) SetHeader(h eventHeader) {
	erfz.header = h
}

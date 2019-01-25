package store

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
)

type actorMoveEvent struct {
	header                                eventHeader
	FromLocationID, ToLocationID, ActorID uuid.UUID
}

func (ame *actorMoveEvent) FromDomain(e core.Event) {
	from := e.(*core.ActorMoveEvent)
	fromID, toID, actorID := from.FromToActorIDs()
	*ame = actorMoveEvent{
		header:         eventHeaderFromDomainEvent(from),
		FromLocationID: fromID,
		ToLocationID:   toID,
		ActorID:        actorID,
	}
}

func (ame actorMoveEvent) ToDomain() core.Event {
	e := core.NewActorMoveEvent(
		ame.FromLocationID,
		ame.ToLocationID,
		ame.ActorID,
		ame.header.AggregateId,
	)
	e.SetSequenceNumber(ame.header.SequenceNumber)
	return e
}

func (ame actorMoveEvent) Header() eventHeader {
	return ame.header
}

func (ame *actorMoveEvent) SetHeader(h eventHeader) {
	ame.header = h
}

type actorAdminRelocateEvent struct {
	header                eventHeader
	ActorID, ToLocationID uuid.UUID
}

func (aare *actorAdminRelocateEvent) FromDomain(e core.Event) {
	from := e.(*core.ActorAdminRelocateEvent)
	*aare = actorAdminRelocateEvent{
		header:       eventHeaderFromDomainEvent(from),
		ActorID:      from.ActorID,
		ToLocationID: from.ToLocationID,
	}
}

func (aare actorAdminRelocateEvent) ToDomain() core.Event {
	e := core.NewActorAdminRelocateEvent(aare.ActorID, aare.ToLocationID, aare.header.AggregateId)
	e.SetSequenceNumber(aare.header.SequenceNumber)
	return e
}

func (aare actorAdminRelocateEvent) Header() eventHeader {
	return aare.header
}

func (aare *actorAdminRelocateEvent) SetHeader(h eventHeader) {
	aare.header = h
}

type actorAddToZoneEvent struct {
	header                      eventHeader
	ActorID, StartingLocationID uuid.UUID
	Name                        string
}

func (aatze *actorAddToZoneEvent) FromDomain(e core.Event) {
	from := e.(*core.ActorAddToZoneEvent)
	*aatze = actorAddToZoneEvent{
		header:             eventHeaderFromDomainEvent(from),
		ActorID:            from.ActorID(),
		StartingLocationID: from.StartingLocationID(),
		Name:               from.Name(),
	}
}

func (aatze actorAddToZoneEvent) ToDomain() core.Event {
	e := core.NewActorAddToZoneEvent(
		aatze.Name,
		aatze.ActorID,
		aatze.StartingLocationID,
		aatze.header.AggregateId,
	)
	e.SetSequenceNumber(aatze.header.SequenceNumber)
	return e
}

func (aatze actorAddToZoneEvent) Header() eventHeader {
	return aatze.header
}

func (aatze *actorAddToZoneEvent) SetHeader(h eventHeader) {
	aatze.header = h
}

type actorRemoveFromZoneEvent struct {
	header  eventHeader
	ActorID uuid.UUID
}

func (arfze *actorRemoveFromZoneEvent) FromDomain(e core.Event) {
	from := e.(*core.ActorRemoveFromZoneEvent)
	*arfze = actorRemoveFromZoneEvent{
		header:  eventHeaderFromDomainEvent(from),
		ActorID: from.ActorID(),
	}
}

func (arfze actorRemoveFromZoneEvent) ToDomain() core.Event {
	e := core.NewActorRemoveFromZoneEvent(arfze.ActorID, arfze.header.AggregateId)
	e.SetSequenceNumber(arfze.header.SequenceNumber)
	return e
}

func (arfze actorRemoveFromZoneEvent) Header() eventHeader {
	return arfze.header
}

func (arfze *actorRemoveFromZoneEvent) SetHeader(h eventHeader) {
	arfze.header = h
}

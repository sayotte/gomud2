package store

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/domain"
)

type actorMoveEvent struct {
	header                                eventHeader
	FromLocationID, ToLocationID, ActorID uuid.UUID
}

func (ame *actorMoveEvent) FromDomain(e domain.Event) {
	from := e.(*domain.ActorMoveEvent)
	fromID, toID, actorID := from.FromToActorIDs()
	*ame = actorMoveEvent{
		header:         eventHeaderFromDomainEvent(from),
		FromLocationID: fromID,
		ToLocationID:   toID,
		ActorID:        actorID,
	}
}

func (ame actorMoveEvent) ToDomain() domain.Event {
	e := domain.NewActorMoveEvent(
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

type actorAddToZoneEvent struct {
	header                      eventHeader
	ActorID, StartingLocationID uuid.UUID
	Name                        string
}

func (aatze *actorAddToZoneEvent) FromDomain(e domain.Event) {
	from := e.(domain.ActorAddToZoneEvent)
	*aatze = actorAddToZoneEvent{
		header:             eventHeaderFromDomainEvent(from),
		ActorID:            from.ActorID(),
		StartingLocationID: from.StartingLocationID(),
		Name:               from.Name(),
	}
}

func (aatze actorAddToZoneEvent) ToDomain() domain.Event {
	e := domain.NewActorAddToZoneEvent(
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

func (arfze *actorRemoveFromZoneEvent) FromDomain(e domain.Event) {
	from := e.(domain.ActorRemoveFromZoneEvent)
	*arfze = actorRemoveFromZoneEvent{
		header:  eventHeaderFromDomainEvent(from),
		ActorID: from.ActorID(),
	}
}

func (arfze actorRemoveFromZoneEvent) ToDomain() domain.Event {
	e := domain.NewActorRemoveFromZoneEvent(arfze.ActorID, arfze.header.AggregateId)
	e.SetSequenceNumber(arfze.header.SequenceNumber)
	return e
}

func (arfze actorRemoveFromZoneEvent) Header() eventHeader {
	return arfze.header
}

func (arfze *actorRemoveFromZoneEvent) SetHeader(h eventHeader) {
	arfze.header = h
}

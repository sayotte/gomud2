package store

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
)

type objectAddToZoneEvent struct {
	header             eventHeader
	ObjectId           uuid.UUID
	Name               string
	StartingLocationId uuid.UUID
}

func (oatze *objectAddToZoneEvent) FromDomain(e core.Event) {
	from := e.(core.ObjectAddToZoneEvent)
	*oatze = objectAddToZoneEvent{
		header: eventHeader{
			EventType:      from.Type(),
			Version:        from.Version(),
			AggregateId:    from.AggregateId(),
			SequenceNumber: from.SequenceNumber(),
		},
		ObjectId:           from.ObjectID(),
		Name:               from.Name(),
		StartingLocationId: from.StartingLocationID(),
	}
}

func (oatze objectAddToZoneEvent) ToDomain() core.Event {
	e := core.NewObjectAddToZoneEvent(
		oatze.Name,
		oatze.ObjectId,
		oatze.StartingLocationId,
		oatze.header.AggregateId,
	)
	e.SetSequenceNumber(oatze.header.SequenceNumber)
	return e
}

func (oatze objectAddToZoneEvent) Header() eventHeader {
	return oatze.header
}

func (oatze *objectAddToZoneEvent) SetHeader(h eventHeader) {
	oatze.header = h
}

type objectRemoveFromZoneEvent struct {
	header   eventHeader
	ObjectID uuid.UUID
	Name     string
}

func (orfze *objectRemoveFromZoneEvent) FromDomain(e core.Event) {
	from := e.(core.ObjectRemoveFromZoneEvent)
	*orfze = objectRemoveFromZoneEvent{
		header:   eventHeaderFromDomainEvent(e),
		ObjectID: from.ObjectID(),
		Name:     from.Name(),
	}
}

func (orfze objectRemoveFromZoneEvent) ToDomain() core.Event {
	e := core.NewObjectRemoveFromZoneEvent(orfze.Name, orfze.ObjectID, orfze.header.AggregateId)
	e.SetSequenceNumber(orfze.header.SequenceNumber)
	return e
}

func (orfze objectRemoveFromZoneEvent) Header() eventHeader {
	return orfze.header
}

func (orfze *objectRemoveFromZoneEvent) SetHeader(h eventHeader) {
	orfze.header = h
}

type objectMoveEvent struct {
	header         eventHeader
	FromLocationId uuid.UUID
	ToLocationID   uuid.UUID
	ObjectID       uuid.UUID
}

func (ome *objectMoveEvent) FromDomain(e core.Event) {
	from := e.(core.ObjectMoveEvent)
	*ome = objectMoveEvent{
		header:         eventHeaderFromDomainEvent(from),
		FromLocationId: from.FromLocationID(),
		ToLocationID:   from.ToLocationID(),
		ObjectID:       from.ObjectID(),
	}
}

func (ome objectMoveEvent) ToDomain() core.Event {
	e := core.NewObjectMoveEvent(
		ome.FromLocationId,
		ome.ToLocationID,
		ome.ObjectID,
		ome.header.AggregateId,
	)
	e.SetSequenceNumber(ome.header.SequenceNumber)
	return e
}

func (ome objectMoveEvent) Header() eventHeader {
	return ome.header
}

func (ome *objectMoveEvent) SetHeader(h eventHeader) {
	ome.header = h
}

type objectAdminRelocateEvent struct {
	header                 eventHeader
	ObjectID, ToLocationID uuid.UUID
}

func (oare *objectAdminRelocateEvent) FromDomain(e core.Event) {
	from := e.(core.ObjectAdminRelocateEvent)
	*oare = objectAdminRelocateEvent{
		header:       eventHeaderFromDomainEvent(from),
		ObjectID:     from.ObjectID,
		ToLocationID: from.ToLocationID,
	}
}

func (oare objectAdminRelocateEvent) ToDomain() core.Event {
	e := core.NewObjectAdminRelocateEvent(oare.ObjectID, oare.ToLocationID, oare.header.AggregateId)
	e.SetSequenceNumber(oare.header.SequenceNumber)
	return e
}

func (oare objectAdminRelocateEvent) Header() eventHeader {
	return oare.header
}

func (oare *objectAdminRelocateEvent) SetHeader(h eventHeader) {
	oare.header = h
}

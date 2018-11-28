package store

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/domain"
)

type objectAddToZoneEvent struct {
	header             eventHeader
	ObjectId           uuid.UUID
	Name               string
	StartingLocationId uuid.UUID
}

func (oatze *objectAddToZoneEvent) FromDomain(e domain.Event) {
	from := e.(domain.ObjectAddToZoneEvent)
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

func (oatze objectAddToZoneEvent) ToDomain() domain.Event {
	e := domain.NewObjectAddToZoneEvent(
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

type objectMoveEvent struct {
	header         eventHeader
	FromLocationId uuid.UUID
	ToLocationID   uuid.UUID
	ObjectID       uuid.UUID
}

func (ome *objectMoveEvent) FromDomain(e domain.Event) {
	from := e.(domain.ObjectMoveEvent)
	*ome = objectMoveEvent{
		header:         eventHeaderFromDomainEvent(from),
		FromLocationId: from.FromLocationID(),
		ToLocationID:   from.ToLocationID(),
		ObjectID:       from.ObjectID(),
	}
}

func (ome objectMoveEvent) ToDomain() domain.Event {
	e := domain.NewObjectMoveEvent(
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

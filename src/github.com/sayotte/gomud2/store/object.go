package store

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
)

type objectAddToZoneEvent struct {
	header                                                   eventHeader
	ObjectId                                                 uuid.UUID
	Name, Description                                        string
	Keywords                                                 []string
	LocationContainerID, ActorContainerID, ObjectContainerID uuid.UUID
	Capacity                                                 int
}

func (oatze *objectAddToZoneEvent) FromDomain(e core.Event) {
	from := e.(*core.ObjectAddToZoneEvent)
	*oatze = objectAddToZoneEvent{
		header:              eventHeaderFromDomainEvent(e),
		ObjectId:            from.ObjectID,
		Name:                from.Name,
		Description:         from.Description,
		Keywords:            from.Keywords,
		LocationContainerID: from.LocationContainerID,
		ActorContainerID:    from.ActorContainerID,
		ObjectContainerID:   from.ObjectContainerID,
		Capacity:            from.Capacity,
	}
}

func (oatze objectAddToZoneEvent) ToDomain() core.Event {
	e := core.NewObjectAddToZoneEvent(
		oatze.Name,
		oatze.Description,
		oatze.Keywords,
		oatze.Capacity,
		oatze.ObjectId,
		oatze.LocationContainerID,
		oatze.ActorContainerID,
		oatze.ObjectContainerID,
		oatze.header.AggregateId,
	)
	e.SetSequenceNumber(oatze.header.SequenceNumber)
	e.SetTimestamp(oatze.header.Timestamp)
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
	from := e.(*core.ObjectRemoveFromZoneEvent)
	*orfze = objectRemoveFromZoneEvent{
		header:   eventHeaderFromDomainEvent(e),
		ObjectID: from.ObjectID,
		Name:     from.Name,
	}
}

func (orfze objectRemoveFromZoneEvent) ToDomain() core.Event {
	e := core.NewObjectRemoveFromZoneEvent(orfze.Name, orfze.ObjectID, orfze.header.AggregateId)
	e.SetSequenceNumber(orfze.header.SequenceNumber)
	e.SetTimestamp(orfze.header.Timestamp)
	return e
}

func (orfze objectRemoveFromZoneEvent) Header() eventHeader {
	return orfze.header
}

func (orfze *objectRemoveFromZoneEvent) SetHeader(h eventHeader) {
	orfze.header = h
}

type objectMoveEvent struct {
	header                                                               eventHeader
	ObjectID                                                             uuid.UUID
	ActorID                                                              uuid.UUID
	FromLocationContainerID, FromActorContainerID, FromObjectContainerID uuid.UUID
	ToLocationContainerID, ToActorContainerID, ToObjectContainerID       uuid.UUID
}

func (ome *objectMoveEvent) FromDomain(e core.Event) {
	from := e.(*core.ObjectMoveEvent)
	*ome = objectMoveEvent{
		header:                  eventHeaderFromDomainEvent(from),
		ObjectID:                from.ObjectID,
		ActorID:                 from.ActorID,
		FromLocationContainerID: from.FromLocationContainerID,
		FromActorContainerID:    from.FromActorContainerID,
		FromObjectContainerID:   from.FromObjectContainerID,
		ToLocationContainerID:   from.ToLocationContainerID,
		ToActorContainerID:      from.ToActorContainerID,
		ToObjectContainerID:     from.ToObjectContainerID,
	}
}

func (ome objectMoveEvent) ToDomain() core.Event {
	e := core.NewObjectMoveEvent(
		ome.ObjectID,
		ome.ActorID,
		ome.header.AggregateId,
	)
	e.FromLocationContainerID = ome.FromLocationContainerID
	e.FromActorContainerID = ome.FromActorContainerID
	e.FromObjectContainerID = ome.FromObjectContainerID
	e.ToLocationContainerID = ome.ToLocationContainerID
	e.ToActorContainerID = ome.ToActorContainerID
	e.ToObjectContainerID = ome.ToObjectContainerID

	e.SetSequenceNumber(ome.header.SequenceNumber)
	e.SetTimestamp(ome.header.Timestamp)
	return e
}

func (ome objectMoveEvent) Header() eventHeader {
	return ome.header
}

func (ome *objectMoveEvent) SetHeader(h eventHeader) {
	ome.header = h
}

type objectAdminRelocateEvent struct {
	header                                                         eventHeader
	ObjectID                                                       uuid.UUID
	ToLocationContainerID, ToActorContainerID, ToObjectContainerID uuid.UUID
}

func (oare *objectAdminRelocateEvent) FromDomain(e core.Event) {
	from := e.(*core.ObjectAdminRelocateEvent)
	*oare = objectAdminRelocateEvent{
		header:                eventHeaderFromDomainEvent(from),
		ObjectID:              from.ObjectID,
		ToLocationContainerID: from.ToLocationContainerID,
		ToActorContainerID:    from.ToActorContainerID,
		ToObjectContainerID:   from.ToObjectContainerID,
	}
}

func (oare objectAdminRelocateEvent) ToDomain() core.Event {
	e := core.NewObjectAdminRelocateEvent(oare.ObjectID, oare.header.AggregateId)
	e.ToLocationContainerID = oare.ToLocationContainerID
	e.ToActorContainerID = oare.ToActorContainerID
	e.ToObjectContainerID = oare.ToObjectContainerID
	e.SetSequenceNumber(oare.header.SequenceNumber)
	e.SetTimestamp(oare.header.Timestamp)

	return e
}

func (oare objectAdminRelocateEvent) Header() eventHeader {
	return oare.header
}

func (oare *objectAdminRelocateEvent) SetHeader(h eventHeader) {
	oare.header = h
}

type objectMigrateInEvent struct {
	header                                                   eventHeader
	ObjectID                                                 uuid.UUID
	Name, Description                                        string
	Keywords                                                 []string
	FromZoneID                                               uuid.UUID
	LocationContainerID, ActorContainerID, ObjectContainerID uuid.UUID
	Capacity                                                 int
}

func (omie *objectMigrateInEvent) FromDomain(e core.Event) {
	from := e.(*core.ObjectMigrateInEvent)
	*omie = objectMigrateInEvent{
		header:              eventHeaderFromDomainEvent(from),
		ObjectID:            from.ObjectID,
		Name:                from.Name,
		Description:         from.Description,
		Keywords:            from.Keywords,
		FromZoneID:          from.FromZoneID,
		LocationContainerID: from.LocationContainerID,
		ActorContainerID:    from.ActorContainerID,
		ObjectContainerID:   from.ObjectContainerID,
		Capacity:            from.Capacity,
	}
}

func (omie objectMigrateInEvent) ToDomain() core.Event {
	e := core.NewObjectMigrateInEvent(
		omie.Name,
		omie.Description,
		omie.Keywords,
		omie.Capacity,
		omie.ObjectID,
		omie.FromZoneID,
		omie.LocationContainerID,
		omie.ActorContainerID,
		omie.ObjectContainerID,
		omie.header.AggregateId,
	)
	e.SetSequenceNumber(omie.header.SequenceNumber)
	e.SetTimestamp(omie.header.Timestamp)
	return e
}

func (omie objectMigrateInEvent) Header() eventHeader {
	return omie.header
}

func (omie *objectMigrateInEvent) SetHeader(h eventHeader) {
	omie.header = h
}

type objectMigrateOutEvent struct {
	header   eventHeader
	ObjectID uuid.UUID
	Name     string
	ToZoneID uuid.UUID
}

func (omoe *objectMigrateOutEvent) FromDomain(e core.Event) {
	from := e.(*core.ObjectMigrateOutEvent)
	*omoe = objectMigrateOutEvent{
		header:   eventHeaderFromDomainEvent(from),
		ObjectID: from.ObjectID,
		Name:     from.Name,
		ToZoneID: from.ToZoneID,
	}
}

func (omoe objectMigrateOutEvent) ToDomain() core.Event {
	e := core.NewObjectMigrateOutEvent(
		omoe.Name,
		omoe.ObjectID,
		omoe.ToZoneID,
		omoe.header.AggregateId,
	)
	e.SetSequenceNumber(omoe.header.SequenceNumber)
	e.SetTimestamp(omoe.header.Timestamp)
	return e
}

func (omoe objectMigrateOutEvent) Header() eventHeader {
	return omoe.header
}

func (omoe *objectMigrateOutEvent) SetHeader(h eventHeader) {
	omoe.header = h
}

package store

import (
	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/core"
)

type zoneSetDefaultLocationEvent struct {
	header     eventHeader
	LocationID uuid.UUID
}

func (zsdle zoneSetDefaultLocationEvent) ToDomain() core.Event {
	e := core.NewZoneSetDefaultLocationEvent(zsdle.LocationID, zsdle.header.AggregateId)
	e.SetSequenceNumber(zsdle.header.SequenceNumber)
	e.SetTimestamp(zsdle.header.Timestamp)
	return e
}

func (zsdle *zoneSetDefaultLocationEvent) FromDomain(e core.Event) {
	from := e.(*core.ZoneSetDefaultLocationEvent)
	*zsdle = zoneSetDefaultLocationEvent{
		header:     eventHeaderFromDomainEvent(from),
		LocationID: from.LocationID,
	}
}

func (zsdle zoneSetDefaultLocationEvent) Header() eventHeader {
	return zsdle.header
}

func (zsdle *zoneSetDefaultLocationEvent) SetHeader(h eventHeader) {
	zsdle.header = h
}

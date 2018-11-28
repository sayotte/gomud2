package wsapi

import (
	"encoding/json"
	"fmt"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/domain"
)

const (
	EventTypeActorMove           = "actor-move"
	EventTypeActorAddToZone      = "actor-add-to-zone"
	EventTypeActorRemoveFromZone = "actor-remove-from-zone"
	//EventTypeLocationAddToZone
	//EventTypeLocationEdgeAddToZone
	EventTypeObjectAddToZone = "object-add-to-zone"
	EventTypeObjectMove      = "object-move"
)

type Event struct {
	EventType string          `json:"eventType"`
	Version   int             `json:"version"`
	ZoneID    uuid.UUID       `json:"zoneID"`
	Body      json.RawMessage `json:"body"`
}

type populateFromDomainer interface {
	populateFromDomain(e domain.Event)
}

func eventFromDomainEvent(from domain.Event) (Event, error) {
	var e Event
	var frommer populateFromDomainer

	e.ZoneID = from.AggregateId()
	// FIXME should probably validate that we know how to handle the version
	// FIXME we're being sent, so we don't say "oh yeah this is a v2 event"
	// FIXME but then neglect to include the new/changed v2 fields
	e.Version = from.Version()

	switch from.Type() {
	case domain.EventTypeActorMove:
		e.EventType = EventTypeActorMove
		frommer = &ActorMoveEventBody{}
	case domain.EventTypeActorAddToZone:
		e.EventType = EventTypeActorAddToZone
		frommer = &ActorAddToZoneEventBody{}
	case domain.EventTypeActorRemoveFromZone:
		e.EventType = EventTypeActorRemoveFromZone
		frommer = &ActorRemoveFromZoneEventBody{}
	case domain.EventTypeObjectAddToZone:
		e.EventType = EventTypeObjectAddToZone
		frommer = &ObjectAddToZoneEventBody{}
	case domain.EventTypeObjectMove:
		e.EventType = EventTypeObjectMove
		frommer = &ObjectMoveEventBody{}
	default:
		return e, fmt.Errorf("unhandled Event type %T", from)
	}

	frommer.populateFromDomain(from)
	bodyBytes, err := json.Marshal(frommer)
	if err != nil {
		return e, fmt.Errorf("json.Marshal(1): %s", err)
	}
	e.Body = bodyBytes

	return e, nil
}

type ActorMoveEventBody struct {
	FromLocationID uuid.UUID `json:"fromLocationID"`
	ToLocationID   uuid.UUID `json:"toLocationID"`
	ActorID        uuid.UUID `json:"actorID"`
}

func (ameb *ActorMoveEventBody) populateFromDomain(e domain.Event) {
	typedEvent := e.(*domain.ActorMoveEvent)
	ameb.FromLocationID, ameb.ToLocationID, ameb.ActorID = typedEvent.FromToActorIDs()
}

type ActorAddToZoneEventBody struct {
	ActorID            uuid.UUID `json:"actorID"`
	Name               string    `json:"name"`
	StartingLocationID uuid.UUID `json:"startingLocationID"`
}

func (aatzeb *ActorAddToZoneEventBody) populateFromDomain(e domain.Event) {
	typedEvent := e.(domain.ActorAddToZoneEvent)
	aatzeb.ActorID = typedEvent.ActorID()
	aatzeb.Name = typedEvent.Name()
	aatzeb.StartingLocationID = typedEvent.StartingLocationID()
}

type ActorRemoveFromZoneEventBody struct {
	ActorID uuid.UUID `json:"actorID"`
}

func (arfzeb *ActorRemoveFromZoneEventBody) populateFromDomain(e domain.Event) {
	typedEvent := e.(domain.ActorRemoveFromZoneEvent)
	arfzeb.ActorID = typedEvent.ActorID()
}

type ObjectAddToZoneEventBody struct {
	ObjectID           uuid.UUID `json:"objectID"`
	Name               string    `json:"name"`
	StartingLocationID uuid.UUID `json:"startingLocationID"`
}

func (oatzeb *ObjectAddToZoneEventBody) populateFromDomain(e domain.Event) {
	typedEvent := e.(domain.ObjectAddToZoneEvent)
	oatzeb.ObjectID = typedEvent.ObjectID()
	oatzeb.Name = typedEvent.Name()
	oatzeb.StartingLocationID = typedEvent.StartingLocationID()
}

type ObjectMoveEventBody struct {
	FromLocationID uuid.UUID `json:"fromLocationID"`
	ToLocationID   uuid.UUID `json:"toLocationID"`
	ObjectID       uuid.UUID `json:"objectID"`
}

func (omeb *ObjectMoveEventBody) populateFromDomain(e domain.Event) {
	typedEvent := e.(domain.ObjectMoveEvent)
	omeb.FromLocationID = typedEvent.FromLocationID()
	omeb.ToLocationID = typedEvent.ToLocationID()
	omeb.ObjectID = typedEvent.ObjectID()
}

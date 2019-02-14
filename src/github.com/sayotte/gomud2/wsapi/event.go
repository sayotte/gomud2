package wsapi

import (
	"encoding/json"
	"fmt"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/core"
)

const (
	EventTypeActorMove           = "actor-move"
	EventTypeActorAddToZone      = "actor-add-to-zone"
	EventTypeActorRemoveFromZone = "actor-remove-from-zone"
	EventTypeActorMigrateIn      = "actor-migrate-in"
	EventTypeActorMigrateOut     = "actor-migrate-out"
	//EventTypeLocationAddToZone
	//EventTypeExitAddToZone
	EventTypeObjectAddToZone      = "object-add-to-zone"
	EventTypeObjectRemoveFromZone = "object-remove-from-zone"
	EventTypeObjectMove           = "object-move"
	EventTypeCombatMeleeDamage    = "combat-melee-damage"
	EventTypeCombatDodge          = "combat-dodge"
)

type Event struct {
	EventType string          `json:"eventType"`
	Version   int             `json:"version"`
	ZoneID    uuid.UUID       `json:"zoneID"`
	Body      json.RawMessage `json:"body"`
}

type populateFromDomainer interface {
	populateFromDomain(e core.Event)
}

func eventFromDomainEvent(from core.Event) (Event, error) {
	var e Event
	var frommer populateFromDomainer

	e.ZoneID = from.AggregateId()
	// FIXME should probably validate that we know how to handle the version
	// FIXME we're being sent, so we don't say "oh yeah this is a v2 event"
	// FIXME but then neglect to include the new/changed v2 fields
	e.Version = from.Version()

	switch from.Type() {
	case core.EventTypeActorMove:
		e.EventType = EventTypeActorMove
		frommer = &ActorMoveEventBody{}
	case core.EventTypeActorAddToZone:
		e.EventType = EventTypeActorAddToZone
		frommer = &ActorAddToZoneEventBody{}
	case core.EventTypeActorRemoveFromZone:
		e.EventType = EventTypeActorRemoveFromZone
		frommer = &ActorRemoveFromZoneEventBody{}
	case core.EventTypeActorMigrateIn:
		e.EventType = EventTypeActorMigrateIn
		frommer = &ActorMigrateInEventBody{}
	case core.EventTypeActorMigrateOut:
		e.EventType = EventTypeActorMigrateOut
		frommer = &ActorMigrateOutEventBody{}
	case core.EventTypeObjectAddToZone:
		e.EventType = EventTypeObjectAddToZone
		frommer = &ObjectAddToZoneEventBody{}
	case core.EventTypeObjectRemoveFromZone:
		e.EventType = EventTypeObjectRemoveFromZone
		frommer = &ObjectRemoveFromZoneEventBody{}
	case core.EventTypeObjectMove:
		e.EventType = EventTypeObjectMove
		frommer = &ObjectMoveEventBody{}
	case core.EventTypeCombatMeleeDamage:
		e.EventType = EventTypeCombatMeleeDamage
		frommer = &CombatMeleeDamageEventBody{}
	case core.EventTypeCombatDodge:
		e.EventType = EventTypeCombatDodge
		frommer = &CombatDodgeEventBody{}
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

func (ameb *ActorMoveEventBody) populateFromDomain(e core.Event) {
	typedEvent := e.(*core.ActorMoveEvent)
	ameb.FromLocationID, ameb.ToLocationID, ameb.ActorID = typedEvent.FromToActorIDs()
}

type ActorAddToZoneEventBody struct {
	ActorID            uuid.UUID `json:"actorID"`
	Name               string    `json:"name"`
	StartingLocationID uuid.UUID `json:"startingLocationID"`
}

func (aatzeb *ActorAddToZoneEventBody) populateFromDomain(e core.Event) {
	typedEvent := e.(*core.ActorAddToZoneEvent)
	aatzeb.ActorID = typedEvent.ActorID
	aatzeb.Name = typedEvent.Name
	aatzeb.StartingLocationID = typedEvent.StartingLocationID
}

type ActorRemoveFromZoneEventBody struct {
	ActorID uuid.UUID `json:"actorID"`
}

func (arfzeb *ActorRemoveFromZoneEventBody) populateFromDomain(e core.Event) {
	typedEvent := e.(*core.ActorRemoveFromZoneEvent)
	arfzeb.ActorID = typedEvent.ActorID
}

type ActorMigrateInEventBody struct {
	ActorID    uuid.UUID `json:"actorID"`
	Name       string    `json:"name"`
	FromLocID  uuid.UUID `json:"fromLocID"`
	FromZoneID uuid.UUID `json:"fromZoneID"`
	ToLocID    uuid.UUID `json:"toLocID"`
}

func (amieb *ActorMigrateInEventBody) populateFromDomain(e core.Event) {
	from := e.(*core.ActorMigrateInEvent)
	*amieb = ActorMigrateInEventBody{
		ActorID:    from.ActorID,
		Name:       from.Name,
		FromLocID:  from.FromLocID,
		FromZoneID: from.FromZoneID,
		ToLocID:    from.ToLocID,
	}
}

type ActorMigrateOutEventBody struct {
	ActorID   uuid.UUID `json:"actorID"`
	FromLocID uuid.UUID `json:"fromLocID"`
	ToZoneID  uuid.UUID `json:"toZoneID"`
	ToLocID   uuid.UUID `json:"toLocID"`
}

func (amoeb *ActorMigrateOutEventBody) populateFromDomain(e core.Event) {
	from := e.(*core.ActorMigrateOutEvent)
	*amoeb = ActorMigrateOutEventBody{
		ActorID:   from.ActorID,
		FromLocID: from.FromLocID,
		ToZoneID:  from.ToZoneID,
		ToLocID:   from.ToLocID,
	}
}

type ObjectAddToZoneEventBody struct {
	ObjectID            uuid.UUID `json:"objectID"`
	Name                string    `json:"name"`
	LocationContainerID uuid.UUID `json:"locationContainerID"`
	ActorContainerID    uuid.UUID `json:"actorContainerID"`
	ObjectContainerID   uuid.UUID `json:"objectContainerID"`
}

func (oatzeb *ObjectAddToZoneEventBody) populateFromDomain(e core.Event) {
	typedEvent := e.(*core.ObjectAddToZoneEvent)
	oatzeb.ObjectID = typedEvent.ObjectID
	oatzeb.Name = typedEvent.Name
	oatzeb.LocationContainerID = typedEvent.LocationContainerID
	oatzeb.ActorContainerID = typedEvent.ActorContainerID
	oatzeb.ObjectContainerID = typedEvent.ObjectContainerID
}

type ObjectRemoveFromZoneEventBody struct {
	ObjectID uuid.UUID `json:"objectID"`
	Name     string    `json:"name"`
}

func (orfzeb *ObjectRemoveFromZoneEventBody) populateFromDomain(e core.Event) {
	typedEvent := e.(*core.ObjectRemoveFromZoneEvent)
	orfzeb.ObjectID = typedEvent.ObjectID
	orfzeb.Name = typedEvent.Name
}

type ObjectMoveEventBody struct {
	ObjectID                uuid.UUID `json:"objectID"`
	FromLocationContainerID uuid.UUID `json:"fromLocationContainerID"`
	FromActorContainerID    uuid.UUID `json:"fromActorContainerID"`
	FromObjectContainerID   uuid.UUID `json:"fromObjectContainerID"`
	ToLocationContainerID   uuid.UUID `json:"toLocationContainerID"`
	ToActorContainerID      uuid.UUID `json:"toActorContainerID"`
	ToObjectContainerID     uuid.UUID `json:"toObjectContainerID"`
}

func (omeb *ObjectMoveEventBody) populateFromDomain(e core.Event) {
	typedEvent := e.(*core.ObjectMoveEvent)
	omeb.ObjectID = typedEvent.ObjectID
	omeb.FromLocationContainerID = typedEvent.FromLocationContainerID
	omeb.FromActorContainerID = typedEvent.FromActorContainerID
	omeb.FromObjectContainerID = typedEvent.FromObjectContainerID
	omeb.ToLocationContainerID = typedEvent.ToLocationContainerID
	omeb.ToActorContainerID = typedEvent.ToActorContainerID
	omeb.ToObjectContainerID = typedEvent.ToObjectContainerID
}

type CombatMeleeDamageEventBody struct {
	DamageType  string    `json:"damageType"`
	AttackerID  uuid.UUID `json:"attackerID"`
	TargetID    uuid.UUID `json:"targetID"`
	PhysicalDmg int       `json:"physicalDmg"`
	StaminaDmg  int       `json:"staminaDmg"`
	FocusDmg    int       `json:"focusDmg"`
}

func (cmdeb *CombatMeleeDamageEventBody) populateFromDomain(e core.Event) {
	from := e.(*core.CombatMeleeDamageEvent)
	*cmdeb = CombatMeleeDamageEventBody{
		DamageType:  from.DamageType,
		AttackerID:  from.AttackerID,
		TargetID:    from.TargetID,
		PhysicalDmg: from.PhysicalDmg,
		StaminaDmg:  from.StaminaDmg,
		FocusDmg:    from.FocusDmg,
	}
}

type CombatDodgeEventBody struct {
	DamageType string
	AttackerID uuid.UUID
	TargetID   uuid.UUID
}

func (cdeb *CombatDodgeEventBody) populateFromDomain(e core.Event) {
	from := e.(*core.CombatDodgeEvent)
	*cdeb = CombatDodgeEventBody{
		DamageType: from.DamageType,
		AttackerID: from.AttackerID,
		TargetID:   from.TargetID,
	}
}

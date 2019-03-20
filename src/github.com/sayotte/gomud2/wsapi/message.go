package wsapi

import (
	"encoding/json"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/commands"
)

type Message struct {
	Type      string
	MessageID uuid.UUID `json:"messageID"`
	Payload   json.RawMessage
}

const (
	MessageTypeProcessingError               = "processing-error"
	MessageTypeListActorsCommand             = "list-actors"
	MessageTypeListActorsComplete            = "actors-list"
	MessageTypeAttachActorCommand            = "attach-actor"
	MessageTypeAttachActorComplete           = "actor-attached"
	MessageTypeMoveActorCommand              = "move-actor"
	MessageTypeMoveActorComplete             = "move-actor-complete"
	MessageTypeLookAtOtherActorCommand       = "look-at-other-actor"
	MessageTypeLookAtOtherActorComplete      = "look-at-other-actor-complete"
	MessageTypeLookAtObjectCommand           = "look-at-object"
	MessageTypeLookAtObjectComplete          = "look-at-object-complete"
	MessageTypeMoveObjectCommand             = "move-object"
	MessageTypeMoveObjectComplete            = "move-object-complete"
	MessageTypeMoveObjectSubcontainer        = "move-object-subcontainer"
	MessageTypeEvent                         = "event"
	MessageTypeGetCurrentLocationInfoCommand = "get-current-location-info"
	MessageTypeCurrentLocationInfoComplete   = "current-location-info"
)

type CompleteListActors struct {
	ActorIDs []uuid.UUID `json:"actorIDs"`
}

type CommandAttachActor struct {
	ActorID uuid.UUID `json:"actorID"`
}

type CompleteAttachActor struct {
	ActorID uuid.UUID `json:"actorID"`
}

type CommandMoveActor struct {
	Direction string
}

type CommandLookAtOtherActor struct {
	ActorID uuid.UUID `json:"actorID"`
}

type CommandLookAtObject struct {
	ObjectID uuid.UUID `json:"objectID"`
}

type CommandMoveObject struct {
	ObjectID       uuid.UUID `json:"objectID"`
	FromLocationID uuid.UUID
	FromActorID    uuid.UUID
	FromObjectID   uuid.UUID
	ToLocationID   uuid.UUID
	ToActorID      uuid.UUID
	ToObjectID     uuid.UUID
	ToSubcontainer string
}

type CurrentLocationInfo commands.LocationInfo

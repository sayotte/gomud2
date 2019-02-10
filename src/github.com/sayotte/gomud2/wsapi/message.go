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

type CurrentLocationInfo commands.LocationInfo

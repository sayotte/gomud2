package intelligence

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/satori/go.uuid"

	uuid2 "github.com/sayotte/gomud2/uuid"
	"github.com/sayotte/gomud2/wsapi"
)

func moveSelf(direction string, msgSender MessageSender, intellect *Intellect) (bool, error) {
	cmd := wsapi.CommandMoveActor{Direction: direction}
	err := sendSyncMessage(wsapi.MessageTypeMoveActorCommand, cmd, msgSender, intellect)
	if err != nil {
		return false, err
	}
	return true, nil
}

func moveObject(
	objID, fromLocID, fromActorID, fromObjID, toLocID, toActorID, toObjID uuid.UUID,
	toSubcontainer string,
	msgSender MessageSender,
	intellect *Intellect,
) error {
	cmd := wsapi.CommandMoveObject{
		ObjectID:       objID,
		FromLocationID: fromLocID,
		FromActorID:    fromActorID,
		FromObjectID:   fromObjID,
		ToLocationID:   toLocID,
		ToActorID:      toActorID,
		ToObjectID:     toObjID,
		ToSubcontainer: toSubcontainer,
	}
	return sendSyncMessage(wsapi.MessageTypeMoveObjectCommand, cmd, msgSender, intellect)
}

func melee(meleeType string, targetID uuid.UUID, msgSender MessageSender, intellect *Intellect) error {
	cmd := wsapi.CommandMeleeCombat{
		AttackType: meleeType,
		TargetID:   targetID,
	}
	return sendSyncMessage(wsapi.MessageTypeMeleeCombatCommand, cmd, msgSender, intellect)
}

func sendSyncMessage(msgType string, payload interface{}, msgSender MessageSender, intellect *Intellect) error {
	msgID := uuid2.NewId()
	waiter := &sync.WaitGroup{}
	waiter.Add(1)
	var err error
	callback := func(msg wsapi.Message) {
		if msg.Type == wsapi.MessageTypeProcessingError {
			err = errors.New(string(msg.Payload))
		}
		waiter.Done()
	}
	intellect.registerResponseCallback(msgID, callback)

	msgPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("json.Marshal: %s", err)
	}
	msg := wsapi.Message{
		Type:      msgType,
		MessageID: msgID,
		Payload:   msgPayload,
	}

	err = msgSender.SendMessage(msg)
	if err != nil {
		return err
	}

	waiter.Wait()
	return nil
}

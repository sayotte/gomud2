package innerbrain

import (
	"encoding/json"
	"fmt"
	uuid2 "github.com/sayotte/gomud2/uuid"
	"github.com/sayotte/gomud2/wsapi"
	"sync"
)

func moveSelf(direction string, senderCallbacker MessageSenderCallbacker) (bool, error) {
	msgId := uuid2.NewId()
	waiter := &sync.WaitGroup{}
	waiter.Add(1)
	var success bool
	callback := func(msg wsapi.Message) {
		if msg.Type == wsapi.MessageTypeMoveActorComplete {
			success = true
		}
		waiter.Done()
	}
	senderCallbacker.RegisterResponseCallback(msgId, callback)

	cmd := wsapi.CommandMoveActor{Direction: direction}
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return false, fmt.Errorf("json.Marshal(CommandMoveActor): %s", err)
	}
	msg := wsapi.Message{
		Type:      wsapi.MessageTypeMoveActorCommand,
		MessageID: msgId,
		Payload:   cmdBytes,
	}
	//startTime := time.Now()
	err = senderCallbacker.SendMessage(msg)
	if err != nil {
		return false, err
	}

	waiter.Wait()
	//fmt.Printf("BRAIN DEBUG: round-trip to move to new location took %s\n", time.Now().Sub(startTime))
	return success, nil
}

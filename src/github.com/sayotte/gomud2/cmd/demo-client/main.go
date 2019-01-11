package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sayotte/gomud2/uuid"
	"github.com/sayotte/gomud2/wsapi"
	"net/http"
)

const (
	username = "a"
	password = "a"
)

func main() {
	h := &http.Header{}
	userPass := username + ":" + password
	b64UserPass := base64.StdEncoding.EncodeToString([]byte(userPass))
	h.Add("Authorization", "Basic "+b64UserPass)
	dialer := &websocket.Dialer{}
	conn, res, err := dialer.Dial("ws://localhost:4001", *h)
	if err != nil {
		panic(err)
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		panic(fmt.Sprintf("res.StatusCode: %d\n", res.StatusCode))
	}
	fmt.Println("Connected / logged in / upgraded to websocket")

	// List available Actor IDs
	requestID := uuid.NewId()
	listActorsMsg := wsapi.Message{
		Type:      wsapi.MessageTypeListActorsCommand,
		MessageID: requestID,
	}
	sendMessage(jsonify(listActorsMsg), conn)
	// Receive response
	rcvdBytes := getMessage(conn)
	var rcvdMsg wsapi.Message
	dejsonify(rcvdBytes, &rcvdMsg)
	if rcvdMsg.Type != wsapi.MessageTypeListActorsComplete {
		panic(fmt.Sprintf("unexpected response type %q", rcvdMsg.Type))
	}
	var actorListMsg wsapi.CompleteListActors
	dejsonify(rcvdMsg.Payload, &actorListMsg)
	if len(actorListMsg.ActorIDs) == 0 {
		panic("no actor IDs returned!")
	}
	fmt.Println("List of Actor IDs returned")

	// Attach to the first Actor ID returned
	requestID = uuid.NewId()
	msgBody := wsapi.CommandAttachActor{
		ActorID: actorListMsg.ActorIDs[0],
	}
	attachActorMsg := wsapi.Message{
		Type:      wsapi.MessageTypeAttachActorCommand,
		MessageID: requestID,
		Payload:   jsonify(msgBody),
	}
	sendMessage(jsonify(attachActorMsg), conn)
	// Receive response
	rcvdBytes = getMessage(conn)
	dejsonify(rcvdBytes, &rcvdMsg)
	if rcvdMsg.Type != wsapi.MessageTypeAttachActorComplete {
		panic(fmt.Sprintf("unexpected response type %q", rcvdMsg.Type))
	}
	fmt.Println("Attached to Actor, dumping events as they come in...")

	// Loop, printing (after pretty-JSONifying) events we receive
	for {
		dejsonify(getMessage(conn), &rcvdMsg)
		if rcvdMsg.Type != wsapi.MessageTypeEvent {
			panic(fmt.Sprintf("expected an Event message, got %q", rcvdMsg.Type))
		}
		var rcvdEvent wsapi.Event
		dejsonify(rcvdMsg.Payload, &rcvdEvent)
		fmt.Printf("\nReceived:\n%s\n", jsonify(rcvdEvent))
	}
}

func sendMessage(body []byte, conn *websocket.Conn) {
	err := conn.WriteMessage(websocket.TextMessage, body)
	if err != nil {
		panic(err)
	}
}

func getMessage(conn *websocket.Conn) []byte {
	_, rcvdBytes, err := conn.ReadMessage()
	if err != nil {
		panic(err)
	}
	return rcvdBytes
}

func jsonify(in interface{}) []byte {
	outBytes, err := json.MarshalIndent(in, "", "    ")
	if err != nil {
		panic(err)
	}
	return outBytes
}

func dejsonify(in []byte, target interface{}) {
	err := json.Unmarshal(in, target)
	if err != nil {
		panic(err)
	}
}

package wsapi

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/commands"
	"github.com/sayotte/gomud2/core"
)

type session struct {
	conn *websocket.Conn

	authService AuthService
	accountID   uuid.UUID
	authZDesc   *auth.AuthZDescriptor

	eventChan    chan core.Event
	sendChan     chan []byte
	sendQueueLen int
	receiveChan  chan Message
	stopChan     chan struct{}
	stopWG       *sync.WaitGroup
	stopOnce     *sync.Once

	world *core.World
	actor *core.Actor
}

func (s *session) start() {
	s.stopOnce = &sync.Once{}
	s.stopChan = make(chan struct{})
	s.eventChan = make(chan core.Event, s.sendQueueLen)
	s.sendChan = make(chan []byte, s.sendQueueLen)
	s.receiveChan = make(chan Message)
	go s.receiveLoop()
	go s.mainLoop()
}

func (s *session) stop() {
	// Have to wrap this in a sync.Once, because due to the
	// sync.WaitGroup.Wait() call the calling goroutines are going to have to
	// call it async, which leaves the door open for processing 1+ additional
	// events/messages that might then in turn call this.
	stopFunc := func() {
		s.stopWG = &sync.WaitGroup{}
		s.stopWG.Add(2)
		close(s.stopChan)
		s.stopWG.Wait()

		// we ignore errors as we send this shutdown message, because we might be stopping
		// due to having already encountered an error writing to the connection
		_ = s.conn.WriteMessage(websocket.CloseNormalClosure, []byte("terminating connection"))
		_ = s.conn.Close()
	}
	s.stopOnce.Do(stopFunc)
}

// SendEvent implements the domain.Observer interface
func (s *session) SendEvent(e core.Event) {
	s.eventChan <- e
}

// Evict implements the domain.Observer interface
func (s *session) Evict() {
	s.sendCloseDetachAndStop(websocket.CloseGoingAway, "evicted from attached actor")
}

func (s *session) receiveLoop() {
	for {
		select {
		case <-s.stopChan:
			s.stopWG.Done()
			return
		default:
		}

		msgType, msgBytes, err := s.conn.ReadMessage()
		if err != nil {
			// we may be getting an error because we're reading from a closed
			// Conn (there's a race condition in calling "go s.stop(); continue")
			// but if not, it's a real error and we should emit it
			if !isAnyWebsocketCloseErrorHelper(err) {
				fmt.Printf("WSAPI ERROR: s.conn.ReadMessage(): %s\n", err)
			}
			go s.stop()
			continue
		}
		switch msgType {
		// Ping has a default handler, none needed here
		// We don't send Pings, so we don't need to handle Pings (blow up in
		//   the default case if we get one)
		// We don't expect/want binary messages, so blow up in the default case
		//   if we get one
		// Respect close messages by shutting down gracefully
		case websocket.CloseMessage:
			go s.stop()
			continue
		// Specify a case for Text messages so they don't fall to the default
		case websocket.TextMessage:
			// ...
		// Blow up if we get anything else as it's not RFC 6455 compliant
		default:
			s.sendCloseDetachAndStop(websocket.CloseUnsupportedData, fmt.Sprintf("unhandleable message type %d", msgType))
			continue
		}

		var msg Message
		err = json.Unmarshal(msgBytes, &msg)
		if err != nil {
			fmt.Printf("WSAPI ERROR: json.Unmarshal(): %s\n", err)
			s.sendCloseDetachAndStop(websocket.ClosePolicyViolation, "message JSON data cannot be decoded")
			continue
		}
		s.receiveChan <- msg
	}
}

func (s *session) mainLoop() {
	for {
		select {
		case <-s.stopChan:
			s.stopWG.Done()
			return
		case e := <-s.eventChan:
			event, err := eventFromDomainEvent(e)
			if err != nil {
				fmt.Printf("WSAPI ERROR: %s\n", err)
				s.sendCloseDetachAndStop(websocket.CloseInternalServerErr, "")
				continue
			}
			s.sendMessage(MessageTypeEvent, event, uuid.Nil)
		case msg := <-s.receiveChan:
			switch msg.Type {
			case MessageTypeAttachActorCommand:
				s.handleCommandAttachActor(msg)
			case MessageTypeListActorsCommand:
				s.handleCommandListActors(msg)
			case MessageTypeMoveActorCommand:
				s.handleCommandMoveActor(msg)
			default:
				fmt.Printf("WSAPI ERROR: session received message of type %q\n", msg.Type)
				s.sendCloseDetachAndStop(websocket.CloseProtocolError, fmt.Sprintf("unhandleable API message type %q", msg.Type))
			}
		}
	}
}

func (s *session) sendCloseDetachAndStop(closeCode int, closeText string) {
	payload := websocket.FormatCloseMessage(closeCode, closeText)
	_ = s.conn.WriteMessage(websocket.CloseMessage, payload)

	if s.actor != nil {
		s.actor.RemoveObserver(s)
		s.actor = nil
	}

	go s.stop()
}

func (s *session) sendMessage(typ string, payload interface{}, id uuid.UUID) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("WSAPI ERROR: json.Marshal(1): %s\n", err)
		s.sendCloseDetachAndStop(websocket.CloseInternalServerErr, "")
		return
	}
	msg := Message{
		Type:      typ,
		MessageID: id,
		Payload:   payloadBytes,
	}
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("WSAPI ERROR: json.Marshal(2): %s\n", err)
		s.sendCloseDetachAndStop(websocket.CloseInternalServerErr, "")
		return
	}
	err = s.conn.WriteMessage(websocket.TextMessage, msgBytes)
	if err != nil {
		if !isAnyWebsocketCloseErrorHelper(err) {
			fmt.Printf("WSAPI ERROR: s.conn.WriteMessage(): %s\n", err)
			s.sendCloseDetachAndStop(websocket.CloseInternalServerErr, "")
		} else {
			go s.stop()
		}
	}
}

func (s *session) handleCommandListActors(msg Message) {
	actorIDs := s.authService.GetActorIDsForAccountID(s.accountID)
	completeMsg := CompleteListActors{
		ActorIDs: actorIDs,
	}
	s.sendMessage(
		MessageTypeListActorsComplete,
		completeMsg,
		msg.MessageID,
	)
}

func (s *session) handleCommandAttachActor(msg Message) {
	if s.actor != nil {
		s.sendMessage(MessageTypeProcessingError, "actor already attached", msg.MessageID)
		return
	}

	var cmd CommandAttachActor
	err := json.Unmarshal(msg.Payload, &cmd)
	if err != nil {
		fmt.Printf("WSAPI: ERROR: json.Unmarshal(): %s\n", err)
		s.sendCloseDetachAndStop(websocket.ClosePolicyViolation, "message JSON data cannot be decoded")
		return
	}

	a := s.world.ActorByID(cmd.ActorID)
	if a == nil {
		errMsg := fmt.Sprintf("actor with ID %q does not exist", cmd.ActorID)
		s.sendMessage(MessageTypeProcessingError, errMsg, msg.MessageID)
		return
	}
	for _, o := range a.Observers() {
		o.Evict()
	}
	a.AddObserver(s)

	s.sendMessage(
		MessageTypeAttachActorComplete,
		CompleteAttachActor{cmd.ActorID},
		msg.MessageID,
	)
}

func (s *session) handleCommandMoveActor(msg Message) {
	var moveCmd CommandMoveActor
	err := json.Unmarshal(msg.Payload, &moveCmd)
	if err != nil {
		fmt.Printf("WSAPI: ERROR: json.Unmarshal(): %s\n", err)
		s.sendCloseDetachAndStop(websocket.ClosePolicyViolation, "message JSON data cannot be decoded")
		return
	}

	newActor, err := commands.MoveActor(s.actor, moveCmd.Direction, s)
	if err != nil {
		if !commands.IsFatalError(err) {
			s.sendMessage(MessageTypeProcessingError, err.Error(), msg.MessageID)
			return
		}
		fmt.Printf("WSAPI ERROR: commands.MoveActor(...): %s\n", err)
		s.sendCloseDetachAndStop(websocket.CloseInternalServerErr, "")
		return
	}
	s.actor = newActor
}

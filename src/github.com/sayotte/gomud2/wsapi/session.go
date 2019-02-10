package wsapi

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/commands"
	"github.com/sayotte/gomud2/core"
)

// This is higher than the maximum send-latency seen running with 400
// Actors in two Locations, with GOMAXPROCS=2, and all Actors moving from
// room to room every 2 seconds. While it's certainly *possible* for this to
// be hit under non-error conditions, it's unlikely and should be quite rare.
// And at the rates at which we'll hit it, it shouldn't be a problem to restart
// the Brain or other client.
//
// On the other hand, it IS likely that when we see latency this high that there
// is a serious problem, and that we're better off cutting-off that client so
// it doesn't continue to block the rest of the system.
var socketWriteTimeoutLen = time.Millisecond * 100

type session struct {
	conn *websocket.Conn

	authService AuthService
	accountID   uuid.UUID
	authZDesc   *auth.AuthZDescriptor

	eventChan        chan core.Event
	lowLevelSendChan chan []byte

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
	s.lowLevelSendChan = make(chan []byte, s.sendQueueLen)
	s.receiveChan = make(chan Message)
	go s.receiveFromClientLoop()
	go s.forwardEventsFromCoreLoop()
	go s.lowLevelSendLoop()
}

// SendEvent implements the domain.Observer interface
func (s *session) SendEvent(e core.Event) {
	//fmt.Printf("WSAPI STATS: eventChan queue-len is %d/%d\n", len(s.eventChan), cap(s.eventChan))
	//select {
	//case s.eventChan <- e:
	//default:
	//fmt.Println("WSAPI ERROR: client event-queue overflowed, terminating\n")
	//s.sendCloseDetachAndStop(websocket.CloseInternalServerErr, "client event-queue overflowed")
	//}
	s.eventChan <- e
}

// Evict implements the domain.Observer interface
func (s *session) Evict() {
	s.sendCloseDetachAndStop(websocket.CloseGoingAway, "evicted from attached actor")
}

func (s *session) lowLevelSendLoop() {
	//timer := metrics.GetOrRegisterTimer("ws-send-latency", metrics.DefaultRegistry)
	for {
		//fmt.Printf("WSAPI STATS: lowLevelSendQueue queue-len is %d/%d\n", len(s.lowLevelSendChan), cap(s.lowLevelSendChan))
		select {
		case <-s.stopChan:
			// drain our intake channel to prevent deadlocks
		drainComplete:
			for {
				select {
				case <-s.lowLevelSendChan:
				default:
					break drainComplete
				}
			}
			s.stopWG.Done()
			return
		case msgBytes := <-s.lowLevelSendChan:
			err := s.conn.SetWriteDeadline(time.Now().Add(socketWriteTimeoutLen))
			if err != nil {
				fmt.Printf("WSAPI ERROR: s.conn.SetWriteDeadline(now + %s): %s\n", socketWriteTimeoutLen, err)
				s.sendCloseDetachAndStop(websocket.CloseAbnormalClosure, "write timeout")
			}
			//start := time.Now()
			err = s.conn.WriteMessage(websocket.TextMessage, msgBytes)
			if err != nil {
				if !isAnyWebsocketCloseErrorHelper(err) {
					fmt.Printf("WSAPI ERROR: s.conn.WriteMessage(): %s\n", err)
					s.sendCloseDetachAndStop(websocket.CloseInternalServerErr, "")
				} else {
					// it was a close error, because we wrote to an already-closed
					// conn, which should never happen
					panic("write to an already closed websocket, maybe?")
				}
			}
			//timer.UpdateSince(start)
		}
	}
}

func (s *session) forwardEventsFromCoreLoop() {
	for {
		select {
		case <-s.stopChan:
			// drain our intake channel to prevent deadlocks
		drainComplete:
			for {
				select {
				case <-s.eventChan:
				default:
					break drainComplete
				}
			}
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
		}
	}
}

func (s *session) receiveFromClientLoop() {
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
			panic("reading from an already closed websocket, maybe?")
		}
		switch msgType {
		// Ping has a default handler, none needed here
		// We don't send Pings, so we don't need to handle Pings (blow up in
		//   the default case if we get one)
		// We don't expect/want binary messages, so blow up in the default case
		//   if we get one
		// Respect close messages by shutting down gracefully
		case websocket.CloseMessage:
			s.sendCloseDetachAndStop(websocket.CloseNormalClosure, "")
		// Specify a case for Text messages so they don't fall to the default
		case websocket.TextMessage:
			var msg Message
			err = json.Unmarshal(msgBytes, &msg)
			if err != nil {
				fmt.Printf("WSAPI ERROR: json.Unmarshal(): %s\n", err)
				s.sendCloseDetachAndStop(websocket.ClosePolicyViolation, "message JSON data cannot be decoded")
				continue
			}
			s.handleMessage(msg)
		// Blow up if we get anything else as it's not RFC 6455 compliant
		default:
			s.sendCloseDetachAndStop(websocket.CloseUnsupportedData, fmt.Sprintf("unhandleable message type %d", msgType))
			continue
		}
	}
}

func (s *session) handleMessage(msg Message) {
	switch msg.Type {
	case MessageTypeAttachActorCommand:
		s.handleCommandAttachActor(msg)
	case MessageTypeListActorsCommand:
		s.handleCommandListActors(msg)
	case MessageTypeMoveActorCommand:
		s.handleCommandMoveActor(msg)
	case MessageTypeGetCurrentLocationInfoCommand:
		s.handleCommandGetCurrentLocInfo(msg)
	default:
		fmt.Printf("WSAPI ERROR: session received message of type %q\n", msg.Type)
		s.sendCloseDetachAndStop(websocket.CloseProtocolError, fmt.Sprintf("unhandleable API message type %q", msg.Type))
	}
}

func (s *session) sendCloseDetachAndStop(closeCode int, closeText string) {
	// Have to wrap this in a sync.Once, because the stopping may cause our
	// intake channels to fill up as we shutdown (we don't care, we're ignoring
	// new work), which will result in this function being called again to
	// handle that "error".
	doStop := func() {
		// detach from our actor so it stops sending us events
		s.actor.RemoveObserver(s)
		s.actor = nil

		// setup a WaitGroup so we can tell when all goroutines have stopped
		s.stopWG = &sync.WaitGroup{}
		s.stopWG.Add(3)
		// stop all the goroutines
		close(s.stopChan)

		// Have to do this async, because we don't want to close the websocket.Conn
		// until our reader/writer goroutines are stopped. Since this function is
		// likely being called by one of those goroutines, we can't block... we must
		// let them iterate to discover that stopChan has been closed. Once they've
		// stopped, we're safe to close the conn.
		doClose := func() {
			s.stopWG.Wait()
			payload := websocket.FormatCloseMessage(closeCode, closeText)
			_ = s.conn.WriteMessage(websocket.CloseMessage, payload)
			_ = s.conn.Close()
		}
		go doClose()
	}
	s.stopOnce.Do(doStop)
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

	//select {
	//case s.lowLevelSendChan <- msgBytes:
	//default:
	//	fmt.Printf("WSAPI ERROR: client send-queue overflowed (%d/%d), terminating\n", len(s.lowLevelSendChan), cap(s.lowLevelSendChan))
	//	s.sendCloseDetachAndStop(websocket.ClosePolicyViolation, "client send-queue overflowed, read messages faster next time")
	//}
	s.lowLevelSendChan <- msgBytes
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
	s.actor = a

	s.sendMessage(
		MessageTypeAttachActorComplete,
		CompleteAttachActor{cmd.ActorID},
		msg.MessageID,
	)
}

func (s *session) handleCommandMoveActor(msg Message) {
	if s.actor == nil {
		return
	}
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
	s.sendMessage(MessageTypeMoveActorComplete, nil, msg.MessageID)
}

func (s *session) handleCommandGetCurrentLocInfo(msg Message) {
	if s.actor == nil {
		return
	}
	lInfo := commands.LookAtLocation(s.actor.Location())
	s.sendMessage(
		MessageTypeCurrentLocationInfoComplete,
		CurrentLocationInfo(lInfo),
		msg.MessageID,
	)
}

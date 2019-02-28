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
	// These three vars have only one writer (a sync.Once function) and only one
	// reader (lowLevelSendLoop()), and access to the variables is sequenced
	// around closing stopChan.
	sendCloseMessage bool
	closeCode        int
	closeText        string

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
	fmt.Println("WSAPI DEBUG: session.Evict(): ...")
	s.sendCloseDetachAndStop(true, websocket.CloseGoingAway, "evicted from attached actor")
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
			if s.sendCloseMessage {
				// Send a close message to our peer, so it shuts down the connection
				payload := websocket.FormatCloseMessage(s.closeCode, s.closeText)
				err := s.conn.SetWriteDeadline(time.Time{})
				if err != nil {
					panic(err)
				}
				err = s.conn.WriteMessage(websocket.CloseMessage, payload)
				if err != nil {
					panic(err)
				}
			}
			// Update the waitgroup, so that the goroutine waiting on that can
			// close the underlying TCP connection.
			s.stopWG.Done()
			return
		case msgBytes := <-s.lowLevelSendChan:
			err := s.conn.SetWriteDeadline(time.Now().Add(socketWriteTimeoutLen))
			if err != nil {
				fmt.Printf("WSAPI ERROR: s.conn.SetWriteDeadline(now + %s): %s\n", socketWriteTimeoutLen, err)
				s.sendCloseDetachAndStop(true, websocket.CloseAbnormalClosure, "write timeout")
			}
			//start := time.Now()
			err = s.conn.WriteMessage(websocket.TextMessage, msgBytes)
			if err != nil {
				if !isAnyWebsocketCloseErrorHelper(err) {
					fmt.Printf("WSAPI ERROR: s.conn.WriteMessage(): %s\n", err)
					s.sendCloseDetachAndStop(true, websocket.CloseInternalServerErr, "")
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
				s.sendCloseDetachAndStop(true, websocket.CloseInternalServerErr, "")
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
			// We may be getting an error because we're reading from a closed
			// Conn (there's a race condition in calling "go s.stop(); continue").
			// We wouldn't have had a chance to catch this, as the underlying
			// websocket.Conn catches the message in-band and starts returning
			// read-errors.
			if !isAnyWebsocketCloseErrorHelper(err) {
				// If not a read-on-closed-conn error, it's a real error and we
				// should complain
				fmt.Printf("WSAPI ERROR: s.conn.ReadMessage(): %s\n", err)
			} else {
				// The underlying connection has closed, but if that was
				// initiated by our peer then we're the first ones to see it,
				// so we need to invoke sendCloseDetachAndStop() so the rest of
				// the goroutines shut down correctly. We specify "false" for
				// "sendCloseMsg" because we've already *received* a Close message
				// (or a dead TCP conn), so sending that is at best superflous.
				s.sendCloseDetachAndStop(false, 0, "")
				// And now we need to keep looping, so that our waitgroup-update
				// code in the select{...} up above is called correctly.
				// Note that stopChan will be closed on our very next iteration
				// through the loop, so we won't hit this again.
				continue
			}
		}
		switch msgType {
		// Ping has a default handler, none needed here
		// We don't send Pings, so we don't need to handle Pings (blow up in
		//   the default case if we get one)
		// We don't expect/want binary messages, so blow up in the default case
		//   if we get one
		// Specify a case for Text messages so they don't fall to the default
		case websocket.TextMessage:
			var msg Message
			err = json.Unmarshal(msgBytes, &msg)
			if err != nil {
				fmt.Printf("WSAPI ERROR: json.Unmarshal(): %s\n", err)
				s.sendCloseDetachAndStop(
					true,
					websocket.ClosePolicyViolation,
					"message JSON data cannot be decoded",
				)
				continue
			}
			s.handleMessage(msg)
		// Blow up if we get anything else as it's not RFC 6455 compliant
		default:
			s.sendCloseDetachAndStop(
				true,
				websocket.CloseUnsupportedData,
				fmt.Sprintf("unhandleable message type %d", msgType),
			)
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
	case MessageTypeLookAtOtherActorCommand:
		s.handleCommandLookAtOtherActor(msg)
	case MessageTypeLookAtObjectCommand:
		s.handleCommandLookAtObject(msg)
	case MessageTypeGetCurrentLocationInfoCommand:
		s.handleCommandGetCurrentLocInfo(msg)
	default:
		fmt.Printf("WSAPI ERROR: session received message of type %q\n", msg.Type)
		s.sendCloseDetachAndStop(true, websocket.CloseProtocolError, fmt.Sprintf("unhandleable API message type %q", msg.Type))
	}
}

func (s *session) sendCloseDetachAndStop(sendCloseMsg bool, closeCode int, closeText string) {
	// Have to wrap this in a sync.Once, because the stopping may cause our
	// intake channels to fill up as we shutdown (we don't care, we're ignoring
	// new work), which will result in this function being called again to
	// handle that "error".
	doStop := func() {
		// detach from our actor so it stops sending us events
		s.actor.RemoveObserver(s)
		s.actor = nil

		// update s.closeCode / s.closeText, so that lowlevelSendLoop() can
		// format and send a Close message to our peer once, which should cause
		// it to close the connection from its side, which will then allow our
		// receiveFromClientLoop() to un-block, freeing the waitgroup below,
		// and finally allowing the doClose() goroutine below to close the
		// underlying TCP connection.
		//
		// Note that we need to do this *before* we close s.stopChan, because
		// as soon as we close that lowLevelSendLoop() is going to read the
		// value of these two variables.
		s.sendCloseMessage = sendCloseMsg
		s.closeCode = closeCode
		s.closeText = closeText

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
		s.sendCloseDetachAndStop(true, websocket.CloseInternalServerErr, "")
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
		s.sendCloseDetachAndStop(true, websocket.CloseInternalServerErr, "")
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
		s.sendCloseDetachAndStop(true, websocket.ClosePolicyViolation, "message JSON data cannot be decoded")
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
		s.sendCloseDetachAndStop(true, websocket.ClosePolicyViolation, "message JSON data cannot be decoded")
		return
	}

	newActor, err := commands.MoveActor(s.actor, moveCmd.Direction, s)
	if err != nil {
		if !commands.IsFatalError(err) {
			s.sendMessage(MessageTypeProcessingError, err.Error(), msg.MessageID)
			return
		}
		fmt.Printf("WSAPI ERROR: commands.MoveActor(...): %s\n", err)
		s.sendCloseDetachAndStop(true, websocket.CloseInternalServerErr, "")
		return
	}
	s.actor = newActor
	s.sendMessage(MessageTypeMoveActorComplete, nil, msg.MessageID)
}

func (s *session) handleCommandLookAtOtherActor(msg Message) {
	var cmd CommandLookAtOtherActor
	err := json.Unmarshal(msg.Payload, &cmd)
	if err != nil {
		fmt.Printf("WSAPI ERROR: json.Unmarshal(): %s\n", err)
		s.sendCloseDetachAndStop(true, websocket.ClosePolicyViolation, "message JSON data cannot be decoded")
		return
	}

	a := s.actor.Zone().ActorByID(cmd.ActorID)
	if a == nil {
		errMsg := fmt.Sprintf("Actor with ID %q does not exist", cmd.ActorID)
		s.sendMessage(MessageTypeProcessingError, errMsg, msg.MessageID)
		return
	}
	if a.Location() != s.actor.Location() {
		s.sendMessage(MessageTypeProcessingError, "too far away", msg.MessageID)
		return
	}

	info := commands.LookAtActor(a)
	s.sendMessage(
		MessageTypeLookAtOtherActorComplete,
		info,
		msg.MessageID,
	)
}

func (s *session) handleCommandLookAtObject(msg Message) {
	var cmd CommandLookAtObject
	err := json.Unmarshal(msg.Payload, &cmd)
	if err != nil {
		fmt.Printf("WSAPI ERROR: json.Unmarshal(): %s\n", err)
		s.sendCloseDetachAndStop(true, websocket.ClosePolicyViolation, "message JSON data cannot be decoded")
		return
	}

	obj := s.actor.Zone().ObjectByID(cmd.ObjectID)
	if obj == nil {
		errMsg := fmt.Sprintf("Object with ID %q does not exist", cmd.ObjectID)
		s.sendMessage(MessageTypeProcessingError, errMsg, msg.MessageID)
		return
	}
	// Object must either be on the ground, or in the top-level of our Actor's inventory
	objLoc := obj.Location()
	objCont := obj.Container()
	if objLoc != s.actor.Location() || (objCont != s.actor && objCont != objLoc) {
		s.sendMessage(MessageTypeProcessingError, "too far away / inside a container", msg.MessageID)
		return
	}

	info := commands.LookAtObject(obj)
	s.sendMessage(
		MessageTypeLookAtObjectComplete,
		info,
		msg.MessageID,
	)
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

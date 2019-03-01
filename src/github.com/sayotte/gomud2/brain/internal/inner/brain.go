package inner

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/brain/internal/inner/intelligence"
	"sync"

	"github.com/sayotte/gomud2/wsapi"
)

func NewBrain(conn *websocket.Conn, actorID uuid.UUID, brainType string) *Brain {
	return &Brain{
		conn:      Connection{WSConn: conn},
		actorID:   actorID,
		brainType: brainType,
	}
}

type Brain struct {
	actorID   uuid.UUID
	brainType string

	intellect *intelligence.Intellect

	conn Connection

	stopChan chan struct{}
	stopOnce *sync.Once
	stopWG   *sync.WaitGroup
}

func (b *Brain) Start() error {
	err := b.attachActorToConn()
	if err != nil {
		return err
	}

	b.intellect = intelligence.LoadIntellect(b.brainType, b, b.actorID)

	b.stopChan = make(chan struct{})
	b.stopOnce = &sync.Once{}

	go b.administrativeLoop()
	go b.readLoop()
	b.intellect.Start()

	return nil
}

func (b *Brain) Shutdown(sendCloseMessage bool, closeCode int, closeText string) {
	b.stopOnce.Do(func() {
		fmt.Println("BRAIN DEBUG: Brain.Shutdown():...")
		b.stopWG = &sync.WaitGroup{}
		b.stopWG.Add(2)
		close(b.stopChan)
		b.intellect.Stop()

		doFinalShutdown := func() {
			b.intellect.BlockUntilStopped()
			if sendCloseMessage {
				b.conn.sendCloseMessage(closeCode, closeText)
			}
			b.stopWG.Wait()
			_ = b.conn.close()
		}
		go doFinalShutdown()
	})
}

func (b *Brain) attachActorToConn() error {
	msgBody := wsapi.CommandAttachActor{
		ActorID: b.actorID,
	}
	bodyBytes, err := json.Marshal(msgBody)
	if err != nil {
		return fmt.Errorf("json.Marshal(msgBody): %s", err)
	}
	attachActorMsg := wsapi.Message{
		Type:      wsapi.MessageTypeAttachActorCommand,
		MessageID: uuid.Nil,
		Payload:   bodyBytes,
	}
	err = b.conn.sendMessage(attachActorMsg)
	if err != nil {
		return err
	}

	_, rcvdBytes, err := b.conn.getLowlevelMessage()
	if err != nil {
		return err
	}
	var rcvdMsg wsapi.Message
	err = json.Unmarshal(rcvdBytes, &rcvdMsg)
	if err != nil {
		return fmt.Errorf("json.Unmarshal(..., rcvdMsg): %s", err)
	}
	if rcvdMsg.Type != wsapi.MessageTypeAttachActorComplete {
		return fmt.Errorf("expected MessageTypeAttachActorComplete, received type %d(?)", rcvdMsg.Type)
	}

	return nil
}

func (b *Brain) SendMessage(msg wsapi.Message) error {
	return b.conn.sendMessage(msg)
}

func (b *Brain) administrativeLoop() {
	// This loop/goroutine exists only to catch errors from our Intellect, and
	// perform shutdown of the readLoop goroutine. Because readLoop() might be
	// blocked on a receive in the meantime, we need this goroutine to catch
	// the error and send a websocket "close" message, which will end up
	// causing our peer to close the connection on their side, which will in
	// turn allow readLoop() to un-block and shutdown.
	for {
		select {
		case <-b.stopChan:
			b.stopWG.Done()
			return
		case err := <-b.intellect.ErrorChan():
			fmt.Printf("BRAIN ERROR: from Intellect: %s\n", err)
			b.Shutdown(true, websocket.CloseInternalServerErr, "")
			continue
		}
	}
}

func (b *Brain) readLoop() {
	for {
		select {
		case <-b.stopChan:
			b.stopWG.Done()
			return
		default:
		}

		msgType, rcvdBytes, err := b.conn.getLowlevelMessage()
		if err != nil {
			fmt.Printf("BRAIN ERROR: %s\n", err)
			b.Shutdown(false, 0, "")
			continue
		}

		if msgType != websocket.TextMessage {
			fmt.Printf("BRAIN ERROR: non-TextMessage from WSAPI, type is %d\n", msgType)
			b.Shutdown(true, websocket.ClosePolicyViolation, fmt.Sprintf("unhandleable message type %d", msgType))
			continue
		}

		var rcvdMsg wsapi.Message
		err = json.Unmarshal(rcvdBytes, &rcvdMsg)
		if err != nil {
			fmt.Printf("BRAIN ERROR: json.Unmarshal(): %s\n", err)
			b.Shutdown(true, websocket.CloseInternalServerErr, "")
			continue
		}
		b.intellect.HandleMessage(rcvdMsg)
	}
}

/* invocation timing thoughts:
- we should be able to re-plan when we have new information
- executing a plan should go as fast as the MUD will allow
- if we re-plan and execute in parallel, we may end up with this scenario:
--- someone hits us as we leave the room; we decide to fight back, but don't see them in the new room
- if we re-plan and execute in lockstep, we may still end up with this scenario:
--- we decide to leave the room; someone hits us as we leave the room, but we don't re-evaluate our
    goal until much later
- if we /potentially/ re-evaluate our goal for every single input, and we goal/plan and execute in
  lockstep, I think we /can/ avoid all timing anomalies
--- by "potentially" I mean that we invoke the code that does planning; it may decide that it's too
    soon to re-plan, or it may decide that a given event is exceptional
--- the process map for this is really complex to think about, though...
- if we merely re-plan after every action we take, and in between take note of exceptional events and
  leave hints for the planner, we could probably

process (***not object!!!***) map:
- messageSenderLoop:
  - selects for messages from other components
  - sends messages on the wire
- messageReceiverLoop:
  - receives messages from the wire
  - forwards received messages to main
- messageMainLoop:
  - selects for:
    - messages from the messager
    - planner/executor CODE: callback registrations from the planner/executor
  - calls planner/executor *code* to update memory based on messages
  - planner/executor CODE: calls previously registered callbacks after processing matching messages
- planExecuteLoop:
  - wakes up on condition variable whenever planner/executor CODE processes a message
  - loops on planning, then executing
    - plan may be kept by planner code, or replaced
    - plan is updated by executor code, and returned for possible re-planning

package/object map:
- messaging
  - sends/receives messages on behalf of planner/executor
- intelligence
  - absorbs messages from messaging, saving observations in its memory
    - calls registered callbacks from intelligence.planExecutLoop
  - plans/executes actions
    - registers callbacks with intelligence.messageLoop
  - sends commands/queries to MUD via messaging
- persistence
  - stores/retrieves observations persistently for intelligence

how are the above things composed?
messaging and intelligence are inter-dependent
  both should depend on interfaces
    intelligence->messaging for testability
    messaging->intelligence so we can swap implementations
intelligence depends on persistence
  intelligence should depend on an interface, for testability and stubbing

memory-- the semantics of it-- is part of the intelligence implementation

*/

/* top-level requirements (in priority order):
- interrupt current activity to react to events / environment
---- e.g. currently in activity "patrol", then someone attacks me
- activity / coarse-grained actions
---- e.g. "patrol" or "follow the leader"
- runtime-configurable context
---- e.g. allies by being hired; enemies by observing an ally being attacked
-
*/

/* top-level implementation structure:
- service which handles loading/attaching AI to an Actor
- "Brain" which handles attached communication to/from Actor, shutting down AI when appropriate etc.
- AI for driving actions to Actor, absorbing information from events/environment
- context store for saving "knowledge" for the AI
*/

/* AI implementation structure:
- utility system for choosing activity
- behavior tree or GOAP for planning/executing activity

events and sensors update context store
AI runs at a minimum interval (to avoid spinning the CPU)
plan-executors block waiting on callbacks, so that execution can proceed as soon as
  a given step has completed, and to allow straight-line programming

how we we interrupt plan-execution when we've chosen a new goal?

*/

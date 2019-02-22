package innerbrain

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/commands"
	"github.com/sayotte/gomud2/wsapi"
)

const defaultIncomingMessageQueueSize = 10

type callbackRegistration struct {
	requestID uuid.UUID
	callback  func(msg wsapi.Message)
}

func NewBrain(conn Connection, actorID uuid.UUID, selector UtilitySelector, executor TrivialExecutor) *Brain {
	return &Brain{
		conn:         conn,
		ActorID:      actorID,
		goalSelector: selector,
		executor:     executor,
	}
}

type Brain struct {
	conn    Connection
	ActorID uuid.UUID
	// Increasing this allows lower level buffers to clear some messages before
	// they're ready to be processed. If it's set low, messages will queue at
	// lower levels, creating back-pressure and eventually causing the server
	// to block on writing to its own buffers (eww). If it's set high, messages
	// will queue on our side, potentially allowing us to get very far behind
	// the rest of the world but not bringing down the server.
	// That said, if we get that far behind, we may never recover and eventually
	// block the server anyway, but the connection between those things may be
	// less obvious.
	//
	// Generally speaking, I have no idea what an ideal value for this is, and
	// it may not even matter... but it's probably important that the server
	// side have some robust buffering and possibly even block-detecting logic
	// which, if necessary, starts dropping messages. Hmm... yes.
	// FIXME change WSAPI code to drop messages rather than block trying to
	// FIXME send them, so that the server doesn't end up doing deeply weird
	// FIXME things just because of a slow-draining client
	IncomingMessageQueueSize int

	memory *Memory

	goalSelector UtilitySelector
	currentGoal  string
	executor     TrivialExecutor

	incomingMessageChan      chan wsapi.Message
	callbackRegistrationChan chan callbackRegistration
	callbacksMap             map[uuid.UUID]func(msg wsapi.Message)
	stopChan                 chan struct{}
	stopOnce                 *sync.Once
}

func (b *Brain) Start() error {
	err := b.attachActorToConn()
	if err != nil {
		return err
	}

	if b.IncomingMessageQueueSize == 0 {
		b.IncomingMessageQueueSize = defaultIncomingMessageQueueSize
	}
	b.incomingMessageChan = make(chan wsapi.Message, b.IncomingMessageQueueSize)

	b.memory = NewMemory(b)

	b.callbackRegistrationChan = make(chan callbackRegistration)
	b.callbacksMap = make(map[uuid.UUID]func(msg wsapi.Message))

	b.stopChan = make(chan struct{})
	b.stopOnce = &sync.Once{}

	b.memory.SetLastMovementTime(time.Now())

	go b.lowLevelReadLoop()
	go b.mainLoop()
	go b.aiLoop()

	return nil
}

func (b *Brain) Shutdown() {
	b.stopOnce.Do(func() {
		_ = b.conn.close()
		close(b.stopChan)
	})
}

func (b *Brain) stopped() bool {
	select {
	case <-b.stopChan:
		return true
	default:
		return false
	}
}

func (b *Brain) attachActorToConn() error {
	msgBody := wsapi.CommandAttachActor{
		ActorID: b.ActorID,
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

func (b *Brain) RegisterResponseCallback(requestID uuid.UUID, callback func(msg wsapi.Message)) {
	b.callbackRegistrationChan <- callbackRegistration{
		requestID: requestID,
		callback:  callback,
	}
}

func (b *Brain) lowLevelReadLoop() {
	for {
		if b.stopped() {
			return
		}

		msgType, rcvdBytes, err := b.conn.getLowlevelMessage()
		if err != nil {
			fmt.Printf("BRAIN ERROR: %s\n", err)
			b.Shutdown()
			return
		}

		if msgType == websocket.CloseMessage {
			b.Shutdown()
			return
		}

		if msgType != websocket.TextMessage {
			fmt.Printf("BRAIN ERROR: non-TextMessage from WSAPI, type is %d\n", msgType)
			b.Shutdown()
			return
		}

		var rcvdMsg wsapi.Message
		err = json.Unmarshal(rcvdBytes, &rcvdMsg)
		if err != nil {
			fmt.Printf("BRAIN ERROR: json.Unmarshal(): %s\n", err)
			b.Shutdown()
			return
		}
		b.incomingMessageChan <- rcvdMsg
	}
}

func (b *Brain) mainLoop() {
	for {
		select {
		case <-b.stopChan:
			return
		case msg := <-b.incomingMessageChan:
			b.handleMessage(msg)
		case registration := <-b.callbackRegistrationChan:
			b.callbacksMap[registration.requestID] = registration.callback
			//for id := range b.callbacksMap {
			//	fmt.Printf("BRAIN DEBUG: callback registered for %q\n", id)
			//}
		}
	}
}

func (b *Brain) handleMessage(msg wsapi.Message) {
	//fmt.Printf("BRAIN DEBUG: handling message of type %q\n", msg.Type)
	switch msg.Type {
	case wsapi.MessageTypeCurrentLocationInfoComplete:
		b.handleCurrentLocInfoMessage(msg)
	case wsapi.MessageTypeEvent:
		b.handleEventMessage(msg)
	default:
		//fmt.Printf("BRAIN WARNING: Brain received message of type %q, no idea what to do with it\n", msg.Type)
	}
	callback, found := b.callbacksMap[msg.MessageID]
	if found {
		callback(msg)
		delete(b.callbacksMap, msg.MessageID)
	}
}

func (b *Brain) handleCurrentLocInfoMessage(msg wsapi.Message) {
	var locInfo commands.LocationInfo
	err := json.Unmarshal(msg.Payload, &locInfo)
	if err != nil {
		fmt.Printf("BRAIN ERROR: json.Unmarshal(locInfo): %s\n", err)
		return
	}

	b.memory.SetLocationInfo(locInfo)
	b.memory.SetCurrentZoneAndLocationID(locInfo.ZoneID, locInfo.ID)
}

func (b *Brain) handleEventMessage(msg wsapi.Message) {
	var eventEnvelope wsapi.Event
	err := json.Unmarshal(msg.Payload, &eventEnvelope)
	if err != nil {
		fmt.Printf("BRAIN ERROR: json.Unmarshal(eventEnvelope): %s\n", err)
		return
	}

	//fmt.Printf("BRAIN DEBUG: event type is %q\n", eventEnvelope.EventType)

	switch eventEnvelope.EventType {
	case wsapi.EventTypeActorMove:
		var e wsapi.ActorMoveEventBody
		err = json.Unmarshal(eventEnvelope.Body, &e)
		if err != nil {
			fmt.Printf("BRAIN ERROR: json.Unmarshal(ActorMoveEventBody): %s\n", err)
			return
		}
		b.handleActorMoveEvent(e, eventEnvelope.ZoneID)
	case wsapi.EventTypeActorDeath:
		var e wsapi.ActorDeathEventBody
		err = json.Unmarshal(eventEnvelope.Body, &e)
		if err != nil {
			fmt.Printf("BRAIN ERROR: json.Unmarshal(ActorDeathEventBody): %s\n", err)
			return
		}
	case wsapi.EventTypeActorMigrateIn:
		var e wsapi.ActorMigrateInEventBody
		err = json.Unmarshal(eventEnvelope.Body, &e)
		if err != nil {
			fmt.Printf("BRAIN ERROR: json.Unmarshal(ActorMigrateInEventBody): %s\n", err)
			return
		}
		b.handleActorMigrateInEvent(e, eventEnvelope.ZoneID)
	case wsapi.EventTypeActorMigrateOut:
		var e wsapi.ActorMigrateOutEventBody
		err = json.Unmarshal(eventEnvelope.Body, &e)
		if err != nil {
			fmt.Printf("BRAIN ERROR: json.Unmarshal(ActorMigrateOutEventBody): %s\n", err)
			return
		}
		b.handleActorMigrateOutEvent(e, eventEnvelope.ZoneID)
	default:
		//fmt.Printf("BRAIN DEBUG: Brain received event of type %q, no idea what to do with it\n", eventEnvelope.EventType)
	}
}

func (b *Brain) handleActorMoveEvent(e wsapi.ActorMoveEventBody, zoneID uuid.UUID) {
	if uuid.Equal(e.ActorID, b.ActorID) {
		// we moved to a new location
		b.memory.SetLastMovementTime(time.Now())
		b.memory.SetCurrentZoneAndLocationID(zoneID, e.ToLocationID)
		b.memory.ClearLocationInfo()
	} else {
		_, currentLocID := b.memory.GetCurrentZoneAndLocationID()
		switch {
		case uuid.Equal(currentLocID, e.FromLocationID):
			// someone left our location
			b.memory.RemoveActorFromLocation(zoneID, e.ToLocationID, e.ActorID)
		case uuid.Equal(currentLocID, e.ToLocationID):
			// someone arrived at our location
			b.memory.AddActorToLocation(zoneID, e.ToLocationID, e.ActorID)
		}
	}
}

func (b *Brain) handleActorDeathEvent(e wsapi.ActorDeathEventBody, zoneID uuid.UUID) {
	// check: did we die?
	if uuid.Equal(e.ActorID, b.ActorID) {
		fmt.Println("BRAIN INFO: our Actor died, shutting down")
		b.Shutdown()
		return
	}
	// else someone else died
	_, currentLocID := b.memory.GetCurrentZoneAndLocationID()
	b.memory.RemoveActorFromLocation(zoneID, currentLocID, e.ActorID)
}

func (b *Brain) handleActorMigrateInEvent(e wsapi.ActorMigrateInEventBody, zoneID uuid.UUID) {
	if uuid.Equal(e.ActorID, b.ActorID) {
		// we migrated to a new zone/location
		// we moved to a new location
		b.memory.SetLastMovementTime(time.Now())
		b.memory.SetCurrentZoneAndLocationID(zoneID, e.ToLocID)
		b.memory.ClearLocationInfo()
	} else {
		// someone migrated in to our location
		b.memory.AddActorToLocation(zoneID, e.ToLocID, e.ActorID)
	}
}

func (b *Brain) handleActorMigrateOutEvent(e wsapi.ActorMigrateOutEventBody, zoneID uuid.UUID) {
	if uuid.Equal(e.ActorID, b.ActorID) {
		// this is weird... we shouldn't witness our own migrate-out event
		fmt.Println("BRAIN WARNING: seeing our own ActorMigrateOutEvent?")
		return
	}
	// someone migrated out of our location
	b.memory.RemoveActorFromLocation(zoneID, e.FromLocID, e.ActorID)
}

func (b *Brain) aiLoop() {
	minDurationBetweenRuns := time.Millisecond * 2000

	ticker := time.NewTicker(minDurationBetweenRuns)

	for {
		if b.stopped() {
			ticker.Stop()
			return
		}

		//fmt.Println("BRAIN DEBUG: doing AI stuff!")
		//start := time.Now()
		b.doAI()
		//runtime := time.Now().Sub(start)
		//fmt.Printf("BRAIN DEBUG: --- did AI stuff in %s ---\n", runtime)

		<-ticker.C
	}
}

func (b *Brain) doAI() {
	newGoal := b.goalSelector.selectGoal(b.memory)
	if newGoal != b.currentGoal {
		//fmt.Printf("BRAIN DEBUG: ===== switching goal from %q -> %q =====\n", b.currentGoal, newGoal)
		b.currentGoal = newGoal
	}

	b.executor.executeGoal(newGoal, b, b.memory)
}

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

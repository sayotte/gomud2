package intelligence

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/commands"
	"github.com/sayotte/gomud2/wsapi"
)

func LoadIntellect(brainType string, msgSender MessageSender, actorID uuid.UUID) *Intellect {
	return &Intellect{
		actorID:   actorID,
		msgSender: msgSender,
	}
}

type Intellect struct {
	actorID uuid.UUID

	memory  *Memory
	planner *planner

	msgSender MessageSender

	callbacksMap      map[uuid.UUID]func(msg wsapi.Message)
	callbacksMapMutex *sync.Mutex

	errChan  chan error
	stopChan chan struct{}
	stopOnce *sync.Once
	stopWG   *sync.WaitGroup
}

func (i *Intellect) Start() {
	i.memory = NewMemory(i.msgSender, i)
	i.memory.SetLastMovementTime(time.Now())

	i.planner = &planner{
		actorID:         i.actorID,
		memory:          i.memory,
		goalSelector:    getStockUtilitySelector(),
		minTimeToRegoal: time.Second / 8,
	}

	i.callbacksMap = make(map[uuid.UUID]func(msg wsapi.Message))
	i.callbacksMapMutex = &sync.Mutex{}

	i.errChan = make(chan error, 1) // needs a buffer of 1 to not block some shutdowns
	i.stopChan = make(chan struct{})
	i.stopOnce = &sync.Once{}
	go i.aiLoop()
}

func (i *Intellect) Stop() {
	i.stopOnce.Do(func() {
		i.stopWG = &sync.WaitGroup{}
		i.stopWG.Add(1)
		close(i.stopChan)

		// Have to unwind all callbacks, or the aiLoop() might remain blocked
		// waiting for a response from the API. We do this by sending a fake
		// error response to all the callbacks. They may (should) return this
		// as an error, which will end up getting sent to i.errorAndStop(),
		// but that stuffs the error into i.errChan which is buffered for
		// exactly this purpose, and then calls Stop() again which is
		// idempotent courtesy of i.stopOnce.
		i.callbacksMapMutex.Lock()
		defer i.callbacksMapMutex.Unlock()
		stopMsgProto := wsapi.Message{
			Type:    wsapi.MessageTypeProcessingError,
			Payload: json.RawMessage([]byte("stopping Intellect")),
		}
		for msgId, callback := range i.callbacksMap {
			stopMsg := stopMsgProto
			stopMsg.MessageID = msgId
			callback(stopMsg)
			delete(i.callbacksMap, msgId)
		}
	})
}

func (i *Intellect) errorAndStop(err error) {
	i.errChan <- err
	i.Stop()
}

func (i *Intellect) ErrorChan() <-chan error {
	return i.errChan
}

func (i *Intellect) BlockUntilStopped() {
	i.stopWG.Wait()
}

func (i *Intellect) registerResponseCallback(requestID uuid.UUID, callback func(msg wsapi.Message)) {
	i.callbacksMapMutex.Lock()
	defer i.callbacksMapMutex.Unlock()
	i.callbacksMap[requestID] = callback
}

func (i *Intellect) HandleMessage(msg wsapi.Message) {
	//fmt.Printf("BRAIN DEBUG: handling message of type %q\n", msg.Type)
	switch msg.Type {
	case wsapi.MessageTypeCurrentLocationInfoComplete:
		i.handleCurrentLocInfoMessage(msg)
	case wsapi.MessageTypeLookAtOtherActorComplete:
		i.handleLookAtOtherActorMessage(msg)
	case wsapi.MessageTypeLookAtObjectComplete:
		i.handleLookAtObjectMessage(msg)
	case wsapi.MessageTypeEvent:
		i.handleEventMessage(msg)
	default:
		//fmt.Printf("BRAIN WARNING: Brain received message of type %q, no idea what to do with it\n", msg.Type)
	}

	i.callbacksMapMutex.Lock()
	defer i.callbacksMapMutex.Unlock()
	callback, found := i.callbacksMap[msg.MessageID]
	if found {
		callback(msg)
		delete(i.callbacksMap, msg.MessageID)
	}
}

func (i *Intellect) handleCurrentLocInfoMessage(msg wsapi.Message) {
	fmt.Println("BRAIN DEBUG: handleCurrentLocInfoMessage(): ...")
	var locInfo commands.LocationInfo
	err := json.Unmarshal(msg.Payload, &locInfo)
	if err != nil {
		fmt.Printf("BRAIN ERROR: json.Unmarshal(locInfo): %s\n", err)
		return
	}

	i.memory.SetLocationInfo(locInfo)
	i.memory.SetCurrentZoneAndLocationID(locInfo.ZoneID, locInfo.ID)
}

func (i *Intellect) handleLookAtOtherActorMessage(msg wsapi.Message) {
	fmt.Println("BRAIN DEBUG: handleLookAtOtherActorMessage(): ...")
	var actorInfo commands.ActorVisibleInfo
	err := json.Unmarshal(msg.Payload, &actorInfo)
	if err != nil {
		fmt.Printf("BRAIN ERROR: json.Unmarshal(actorInfo): %s\n", err)
		return
	}

	i.memory.SetActorInfo(actorInfo)
}

func (i *Intellect) handleLookAtObjectMessage(msg wsapi.Message) {
	fmt.Println("BRAIN DEBUG: handleLookAtObjectMessage(): ...")
	var objectInfo commands.ObjectVisibleInfo
	err := json.Unmarshal(msg.Payload, &objectInfo)
	if err != nil {
		fmt.Printf("BRAIN ERROR: json.Unmarshal(objectInfo): %s\n", err)
		return
	}

	i.memory.SetObjectInfo(objectInfo)
}

func (i *Intellect) handleEventMessage(msg wsapi.Message) {
	var eventEnvelope wsapi.Event
	err := json.Unmarshal(msg.Payload, &eventEnvelope)
	if err != nil {
		fmt.Printf("BRAIN ERROR: json.Unmarshal(eventEnvelope): %s\n", err)
		return
	}

	fmt.Printf("BRAIN DEBUG: event type is %q\n", eventEnvelope.EventType)

	switch eventEnvelope.EventType {
	case wsapi.EventTypeActorMove:
		var e wsapi.ActorMoveEventBody
		err = json.Unmarshal(eventEnvelope.Body, &e)
		if err != nil {
			fmt.Printf("BRAIN ERROR: json.Unmarshal(ActorMoveEventBody): %s\n", err)
			return
		}
		i.handleActorMoveEvent(e, eventEnvelope.ZoneID)
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
		i.handleActorMigrateInEvent(e, eventEnvelope.ZoneID)
	case wsapi.EventTypeActorMigrateOut:
		var e wsapi.ActorMigrateOutEventBody
		err = json.Unmarshal(eventEnvelope.Body, &e)
		if err != nil {
			fmt.Printf("BRAIN ERROR: json.Unmarshal(ActorMigrateOutEventBody): %s\n", err)
			return
		}
		i.handleActorMigrateOutEvent(e, eventEnvelope.ZoneID)
	case wsapi.EventTypeObjectAddToZone:
		var e wsapi.ObjectAddToZoneEventBody
		err = json.Unmarshal(eventEnvelope.Body, &e)
		if err != nil {
			fmt.Printf("BRAIN ERROR: json.Unmarshal(ObjectAddToZoneEventBody): %s\n", err)
			return
		}
		i.handleObjectAddToZoneEvent(e, eventEnvelope.ZoneID)
	case wsapi.EventTypeObjectMove:
		var e wsapi.ObjectMoveEventBody
		err = json.Unmarshal(eventEnvelope.Body, &e)
		if err != nil {
			fmt.Printf("BRAIN ERROR: json.Unmarshal(ObjectMoveEventBody): %s\n", err)
			return
		}
		i.handleObjectMoveEvent(e, eventEnvelope.ZoneID)
	case wsapi.EventTypeCombatMeleeDamage:
		var e wsapi.CombatMeleeDamageEventBody
		err = json.Unmarshal(eventEnvelope.Body, &e)
		if err != nil {
			fmt.Printf("BRAIN ERROR: json.Unmarshal(EventTypeCombatMeleeDamage): %s\n", err)
			return
		}
		i.handleCombatMeleeDamageEvent(e)
	default:
		//fmt.Printf("BRAIN DEBUG: Brain received event of type %q, no idea what to do with it\n", eventEnvelope.EventType)
	}
}

func (i *Intellect) handleActorMoveEvent(e wsapi.ActorMoveEventBody, zoneID uuid.UUID) {
	if uuid.Equal(e.ActorID, i.actorID) {
		// we moved to a new location
		i.memory.SetLastMovementTime(time.Now())
		i.memory.SetCurrentZoneAndLocationID(zoneID, e.ToLocationID)
		i.memory.ClearLocationInfo()
	} else {
		_, currentLocID := i.memory.GetCurrentZoneAndLocationID()
		switch {
		case uuid.Equal(currentLocID, e.FromLocationID):
			// someone left our location
			i.memory.RemoveActorFromLocation(zoneID, e.ToLocationID, e.ActorID)
		case uuid.Equal(currentLocID, e.ToLocationID):
			// someone arrived at our location
			i.memory.AddActorToLocation(zoneID, e.ToLocationID, e.ActorID)
		}
	}
}

func (i *Intellect) handleActorDeathEvent(e wsapi.ActorDeathEventBody, zoneID uuid.UUID) {
	// check: did we die?
	if uuid.Equal(e.ActorID, i.actorID) {
		fmt.Println("BRAIN INFO: our Actor died, shutting down")
		i.Stop()
		return
	}
	// else someone else died
	_, currentLocID := i.memory.GetCurrentZoneAndLocationID()
	i.memory.RemoveActorFromLocation(zoneID, currentLocID, e.ActorID)
}

func (i *Intellect) handleActorMigrateInEvent(e wsapi.ActorMigrateInEventBody, zoneID uuid.UUID) {
	if uuid.Equal(e.ActorID, i.actorID) {
		// we migrated to a new zone/location
		// we moved to a new location
		i.memory.SetLastMovementTime(time.Now())
		i.memory.SetCurrentZoneAndLocationID(zoneID, e.ToLocID)
		i.memory.ClearLocationInfo()
	} else {
		// someone migrated in to our location
		i.memory.AddActorToLocation(zoneID, e.ToLocID, e.ActorID)
	}
}

func (i *Intellect) handleActorMigrateOutEvent(e wsapi.ActorMigrateOutEventBody, zoneID uuid.UUID) {
	if uuid.Equal(e.ActorID, i.actorID) {
		// this is weird... we shouldn't witness our own migrate-out event
		fmt.Println("BRAIN WARNING: seeing our own ActorMigrateOutEvent?")
		return
	}
	// someone migrated out of our location
	i.memory.RemoveActorFromLocation(zoneID, e.FromLocID, e.ActorID)
}

func (i *Intellect) handleObjectAddToZoneEvent(e wsapi.ObjectAddToZoneEventBody, zoneID uuid.UUID) {
	// Actual object info can't be safely pulled from this goroutine, since
	// we'll end up blocking waiting for a callback from ourselves. It will
	// be pulled just-in-time when it's needed anyway.

	switch {
	case !uuid.Equal(e.LocationContainerID, uuid.Nil):
		i.memory.AddObjectToLocation(zoneID, e.LocationContainerID, e.ObjectID)
	case !uuid.Equal(e.ActorContainerID, uuid.Nil):
		// FIXME implement ActorVisibleInfo inventory
	case !uuid.Equal(e.ObjectContainerID, uuid.Nil):
		i.memory.AddObjectToObject(e.ObjectID, e.ObjectContainerID)
	}
}

func (i *Intellect) handleObjectMoveEvent(e wsapi.ObjectMoveEventBody, zoneID uuid.UUID) {
	switch {
	case !uuid.Equal(e.FromLocationContainerID, uuid.Nil):
		i.memory.RemoveObjectFromLocation(zoneID, e.FromLocationContainerID, e.ObjectID)
	case !uuid.Equal(e.FromActorContainerID, uuid.Nil):
		// FIXME implement ActorVisibleInfo inventory
	case !uuid.Equal(e.FromObjectContainerID, uuid.Nil):
		i.memory.RemoveObjectFromObject(e.ObjectID, e.FromObjectContainerID)
	}

	switch {
	case !uuid.Equal(e.ToLocationContainerID, uuid.Nil):
		i.memory.AddObjectToLocation(zoneID, e.ToLocationContainerID, e.ObjectID)
	case !uuid.Equal(e.ToActorContainerID, uuid.Nil):
		// FIXME implement ActorVisibleInfo inventory
	case !uuid.Equal(e.ToObjectContainerID, uuid.Nil):
		i.memory.AddObjectToObject(e.ObjectID, e.ToObjectContainerID)
	}
}

func (i *Intellect) handleCombatMeleeDamageEvent(e wsapi.CombatMeleeDamageEventBody) {
	if uuid.Equal(e.TargetID, i.actorID) {
		i.memory.SetLastAttackedTime(time.Now())
		i.memory.SetLastAttacker(ActorIDTyp(e.AttackerID))
	}
}

func (i *Intellect) aiLoop() {
	minDurationBetweenRuns := time.Millisecond * 5000

	ticker := time.NewTicker(minDurationBetweenRuns)

	var plan executionPlan
	// set initial non-nil value for executionPlan to avoid a panic
	plan = &trivialPlan{
		goalName: "initial-plan",
		memory:   i.memory,
	}

	for {
		select {
		case <-i.stopChan:
			i.stopWG.Done()
			return
		default:
		}

		plan := i.planner.generatePlan(plan)
		plan.executeStep(i.msgSender, i)

		<-ticker.C
	}
}

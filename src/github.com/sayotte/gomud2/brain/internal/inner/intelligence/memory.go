package intelligence

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sayotte/gomud2/commands"
	uuid2 "github.com/sayotte/gomud2/uuid"
	"github.com/sayotte/gomud2/wsapi"
	"math"
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

func NewMemory(msgSender MessageSender, intellect *Intellect) *Memory {
	return &Memory{
		lock:       &sync.RWMutex{},
		localStore: make(map[string]json.Marshaler),
		msgSender:  msgSender,
		intellect:  intellect,
	}
}

type Memory struct {
	lock       *sync.RWMutex
	localStore map[string]json.Marshaler
	msgSender  MessageSender
	intellect  *Intellect
}

func (m *Memory) GetSecondsSinceLastMove() float64 {
	m.lock.Lock()
	defer m.lock.Unlock()
	val, found := m.localStore[memoryLastMovementTimestamp]
	if !found {
		m.localStore[memoryLastMovementTimestamp] = time.Now()
		return 0.0
	}
	lastMoveTS := val.(time.Time)
	return time.Since(lastMoveTS).Seconds()
}

func (m *Memory) SetLastMovementTime(t time.Time) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.localStore[memoryLastMovementTimestamp] = t
}

func (m *Memory) GetSecondsSinceLastAttacked() float64 {
	m.lock.Lock()
	defer m.lock.Unlock()
	val, found := m.localStore[memoryLastAttackedTimestamp]
	if !found {
		m.localStore[memoryLastAttackedTimestamp] = time.Time{}
		return math.MaxFloat64
	}
	lastAttackedTS := val.(time.Time)
	return time.Since(lastAttackedTS).Seconds()
}

func (m *Memory) SetLastAttackedTime(t time.Time) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.localStore[memoryLastAttackedTimestamp] = t
}

func (m *Memory) SetLastAttacker(attackerID ActorIDTyp) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.localStore[memoryLastAttackerID] = attackerID
}

func (m *Memory) GetLastAttacker() ActorIDTyp {
	m.lock.RLock()
	defer m.lock.RUnlock()
	val, found := m.localStore[memoryLastAttackerID]
	if !found {
		return ActorIDTyp(uuid.Nil)
	}
	return val.(ActorIDTyp)
}

// Location data

func (m *Memory) GetCurrentZoneAndLocationID() (uuid.UUID, uuid.UUID) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	var locID, zoneID uuid.UUID

	val, found := m.localStore[memoryMyCurrentZoneID]
	if found {
		stringVal := string(val.(jsonString))
		zoneID = uuid.FromStringOrNil(stringVal)
	}

	val, found = m.localStore[memoryMyCurrentLocationID]
	if found {
		stringVal := string(val.(jsonString))
		locID = uuid.FromStringOrNil(stringVal)
	}

	return zoneID, locID
}

func (m *Memory) SetCurrentZoneAndLocationID(zoneID, locID uuid.UUID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	stringVal := jsonString(zoneID.String())
	m.localStore[memoryMyCurrentZoneID] = stringVal

	stringVal = jsonString(locID.String())
	m.localStore[memoryMyCurrentLocationID] = stringVal
}

func (m *Memory) GetNumActorsInLocation(zoneID, locID uuid.UUID) float64 {
	locInfo, _ := m.GetLocationInfo(zoneID, locID)
	return float64(len(locInfo.Actors))
}

func (m *Memory) GetLocationInfo(zoneID, locID uuid.UUID) (commands.LocationInfo, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	var fullMap map[uuid.UUID]map[uuid.UUID]locInfoEntry

	getZoneSubMap := func() map[uuid.UUID]locInfoEntry {
		val, found := m.localStore[memoryZoneLocInfoMap]
		if found {
			fullMap = map[uuid.UUID]map[uuid.UUID]locInfoEntry(val.(jsonZoneInfoMap))
		} else {
			fullMap = make(map[uuid.UUID]map[uuid.UUID]locInfoEntry)
		}

		zoneSubMap, found := fullMap[zoneID]
		if !found {
			zoneSubMap = make(map[uuid.UUID]locInfoEntry)
		}
		return zoneSubMap
	}
	zoneSubMap := getZoneSubMap()

	var err error
	locInfo, found := zoneSubMap[locID]
	if !found {
		msgID := uuid2.NewId()
		waiter := &sync.WaitGroup{}
		waiter.Add(1)

		callback := func(msg wsapi.Message) {
			m.lock.RLock()
			if msg.Type == wsapi.MessageTypeProcessingError {
				err = errors.New(string(msg.Payload))
			} else {
				zoneSubMap = getZoneSubMap()
				locInfo = zoneSubMap[locID]
			}
			waiter.Done()
		}

		// have to unlock here so that if the brain.mainLoop() is already handling
		// a message that requires writing to the Memory, it can un-block and then
		// get around to accepting our callback registration
		m.lock.RUnlock()
		m.intellect.registerResponseCallback(msgID, callback)

		getLocInfoMsg := wsapi.Message{
			Type:      wsapi.MessageTypeGetCurrentLocationInfoCommand,
			MessageID: msgID,
		}
		err = m.msgSender.SendMessage(getLocInfoMsg)
		if err != nil {
			return locInfo.Info, err
		}

		waiter.Wait()
	}
	return locInfo.Info, err

}

func (m *Memory) SetLocationInfo(info commands.LocationInfo) {
	m.lock.Lock()
	defer m.lock.Unlock()

	var fullMap map[uuid.UUID]map[uuid.UUID]locInfoEntry
	val, found := m.localStore[memoryZoneLocInfoMap]
	if found {
		fullMap = map[uuid.UUID]map[uuid.UUID]locInfoEntry(val.(jsonZoneInfoMap))
	} else {
		fullMap = make(map[uuid.UUID]map[uuid.UUID]locInfoEntry)
	}

	zoneSubMap, found := fullMap[info.ZoneID]
	if !found {
		zoneSubMap = make(map[uuid.UUID]locInfoEntry)
	}

	zoneSubMap[info.ID] = locInfoEntry{
		Info:      info,
		Timestamp: time.Now(),
	}
	fullMap[info.ZoneID] = zoneSubMap
	m.localStore[memoryZoneLocInfoMap] = jsonZoneInfoMap(fullMap)
}

func (m *Memory) ClearLocationInfo() {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.localStore, memoryZoneLocInfoMap)
}

func (m *Memory) RemoveActorFromLocation(zoneID, locID, actorID uuid.UUID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	val, found := m.localStore[memoryZoneLocInfoMap]
	if !found {
		return
	}
	fullMap := map[uuid.UUID]map[uuid.UUID]locInfoEntry(val.(jsonZoneInfoMap))
	zoneSubMap, found := fullMap[zoneID]
	if !found {
		return
	}
	entry, found := zoneSubMap[locID]
	if !found {
		return
	}
	entry.Info.Actors = uuid2.UUIDList(entry.Info.Actors).Remove(actorID)

	zoneSubMap[locID] = entry
	fullMap[zoneID] = zoneSubMap
	m.localStore[memoryZoneLocInfoMap] = jsonZoneInfoMap(fullMap)
}

func (m *Memory) AddActorToLocation(zoneID, locID, actorID uuid.UUID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	val, found := m.localStore[memoryZoneLocInfoMap]
	if !found {
		return
	}
	fullMap := map[uuid.UUID]map[uuid.UUID]locInfoEntry(val.(jsonZoneInfoMap))
	zoneSubMap, found := fullMap[zoneID]
	if !found {
		return
	}
	entry, found := zoneSubMap[locID]
	if !found {
		return
	}
	entry.Info.Actors = append(entry.Info.Actors, actorID)

	zoneSubMap[locID] = entry
	fullMap[zoneID] = zoneSubMap
	m.localStore[memoryZoneLocInfoMap] = jsonZoneInfoMap(fullMap)
}

func (m *Memory) RemoveObjectFromLocation(zoneID, locID, objectID uuid.UUID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	val, found := m.localStore[memoryZoneLocInfoMap]
	if !found {
		return
	}
	fullMap := map[uuid.UUID]map[uuid.UUID]locInfoEntry(val.(jsonZoneInfoMap))
	zoneSubMap, found := fullMap[zoneID]
	if !found {
		return
	}
	entry, found := zoneSubMap[locID]
	if !found {
		return
	}
	entry.Info.Objects = uuid2.UUIDList(entry.Info.Objects).Remove(objectID)

	zoneSubMap[locID] = entry
	fullMap[zoneID] = zoneSubMap
	m.localStore[memoryZoneLocInfoMap] = jsonZoneInfoMap(fullMap)
}

func (m *Memory) AddObjectToLocation(zoneID, locID, objectID uuid.UUID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	val, found := m.localStore[memoryZoneLocInfoMap]
	if !found {
		return
	}
	fullMap := map[uuid.UUID]map[uuid.UUID]locInfoEntry(val.(jsonZoneInfoMap))
	zoneSubMap, found := fullMap[zoneID]
	if !found {
		return
	}
	entry, found := zoneSubMap[locID]
	if !found {
		return
	}
	entry.Info.Objects = append(entry.Info.Objects, objectID)

	zoneSubMap[locID] = entry
	fullMap[zoneID] = zoneSubMap
	m.localStore[memoryZoneLocInfoMap] = jsonZoneInfoMap(fullMap)
}

// Actor data

func (m *Memory) GetActorInfo(actorID uuid.UUID) (commands.ActorVisibleInfo, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	getActorInfoMap := func() actorInfoMap {
		var infoMap actorInfoMap
		val, found := m.localStore[memoryActorInfoMap]
		if found {
			infoMap = val.(actorInfoMap)
		} else {
			infoMap = make(actorInfoMap)
		}
		return infoMap
	}
	infoMap := getActorInfoMap()

	var err error
	actorInfoEnt, found := infoMap[ActorIDTyp(actorID)]
	if !found {
		msgID := uuid2.NewId()
		waiter := &sync.WaitGroup{}
		waiter.Add(1)

		callback := func(msg wsapi.Message) {
			m.lock.RLock()
			if msg.Type == wsapi.MessageTypeProcessingError {
				err = errors.New(string(msg.Payload))
			} else {
				infoMap = getActorInfoMap()
				actorInfoEnt = infoMap[ActorIDTyp(actorID)]
			}
			waiter.Done()
		}

		// have to unlock here so that if the brain.mainLoop() is already handling
		// a message that requires writing to the Memory, it can un-block and then
		// get around to accepting our callback registration
		m.lock.RUnlock()
		m.intellect.registerResponseCallback(msgID, callback)

		cmd := wsapi.CommandLookAtOtherActor{
			ActorID: actorID,
		}
		msgPayload, err := json.Marshal(cmd)
		if err != nil {
			return actorInfoEnt.Info, fmt.Errorf("json.Marshal(msgPayload): %s", err)
		}
		lookAtActorMsg := wsapi.Message{
			Type:      wsapi.MessageTypeLookAtOtherActorCommand,
			MessageID: msgID,
			Payload:   msgPayload,
		}

		err = m.msgSender.SendMessage(lookAtActorMsg)
		if err != nil {
			return actorInfoEnt.Info, err
		}
		waiter.Wait()
	}

	return actorInfoEnt.Info, err
}

func (m *Memory) SetActorInfo(info commands.ActorVisibleInfo) {
	m.lock.Lock()
	defer m.lock.Unlock()

	var infoMap actorInfoMap
	val, found := m.localStore[memoryActorInfoMap]
	if found {
		infoMap = val.(actorInfoMap)
	} else {
		infoMap = make(actorInfoMap)
	}

	infoMap[ActorIDTyp(info.ID)] = actorInfoEntry{
		Info:      info,
		Timestamp: time.Now(),
	}
	m.localStore[memoryActorInfoMap] = infoMap
}

// Object data

func (m *Memory) SetObjectInfo(info commands.ObjectVisibleInfo) {
	m.lock.Lock()
	defer m.lock.Unlock()

	var infoMap objectInfoMap
	val, found := m.localStore[memoryObjectInfoMap]
	if found {
		infoMap = val.(objectInfoMap)
	} else {
		infoMap = make(objectInfoMap)
	}

	infoMap[objectIDTyp(info.ID)] = objectInfoEntry{
		Info:      info,
		Timestamp: time.Now(),
	}
	m.localStore[memoryObjectInfoMap] = infoMap
}

func (m *Memory) GetObjectInfo(objectID uuid.UUID) (commands.ObjectVisibleInfo, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	getObjectInfoMap := func() objectInfoMap {
		var infoMap objectInfoMap
		val, found := m.localStore[memoryObjectInfoMap]
		if found {
			infoMap = val.(objectInfoMap)
		} else {
			infoMap = make(objectInfoMap)
		}
		return infoMap
	}
	infoMap := getObjectInfoMap()

	var err error
	objectInfoEnt, found := infoMap[objectIDTyp(objectID)]
	if !found {
		msgID := uuid2.NewId()
		waiter := &sync.WaitGroup{}
		waiter.Add(1)

		callback := func(msg wsapi.Message) {
			m.lock.RLock()
			if msg.Type == wsapi.MessageTypeProcessingError {
				err = errors.New(string(msg.Payload))
			} else {
				infoMap = getObjectInfoMap()
				objectInfoEnt = infoMap[objectIDTyp(objectID)]
			}
			waiter.Done()
		}

		// have to unlock here so that if the brain.mainLoop() is already handling
		// a message that requires writing to the Memory, it can un-block and then
		// get around to accepting our callback registration
		m.lock.RUnlock()
		m.intellect.registerResponseCallback(msgID, callback)

		cmd := wsapi.CommandLookAtObject{
			ObjectID: objectID,
		}
		msgPayload, err := json.Marshal(cmd)
		if err != nil {
			return objectInfoEnt.Info, fmt.Errorf("json.Marshal(msgPayload): %s", err)
		}
		lookAtObjectMsg := wsapi.Message{
			Type:      wsapi.MessageTypeLookAtObjectCommand,
			MessageID: msgID,
			Payload:   msgPayload,
		}
		err = m.msgSender.SendMessage(lookAtObjectMsg)
		if err != nil {
			return objectInfoEnt.Info, err
		}
		waiter.Wait()
	}

	return objectInfoEnt.Info, err
}

func (m *Memory) RemoveObjectFromObject(objectID, fromObjID uuid.UUID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	val, found := m.localStore[memoryObjectInfoMap]
	if !found {
		return
	}
	infoMap := val.(objectInfoMap)

	objectInfoEnt, found := infoMap[objectIDTyp(fromObjID)]
	if !found {
		return
	}

	objList := objectInfoEnt.Info.ContainedObjects
	objList = uuid2.UUIDList(objList).Remove(objectID)

	objectInfoEnt.Info.ContainedObjects = objList
	infoMap[objectIDTyp(fromObjID)] = objectInfoEnt
}

func (m *Memory) AddObjectToObject(objectID, toObjID uuid.UUID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	val, found := m.localStore[memoryObjectInfoMap]
	if !found {
		return
	}
	infoMap := val.(objectInfoMap)

	objectInfoEnt, found := infoMap[objectIDTyp(toObjID)]
	if !found {
		return
	}

	objList := objectInfoEnt.Info.ContainedObjects
	objList = uuid2.UUIDList(objList).Remove(objectID)

	objectInfoEnt.Info.ContainedObjects = objList
	infoMap[objectIDTyp(toObjID)] = objectInfoEnt
}

// Derived queries

func (m *Memory) IsWeaponOnGround() bool {
	currentZoneID, currentLocID := m.GetCurrentZoneAndLocationID()
	locInfo, err := m.GetLocationInfo(currentZoneID, currentLocID)
	if err != nil {
		fmt.Printf("BRAIN ERROR: %s\n", err)
		return false
	}
	for _, objID := range locInfo.Objects {
		objInfo, err := m.GetObjectInfo(objID)
		if err != nil {
			fmt.Printf("BRAIN ERROR: %s\n", err)
			return false
		}
		switch {
		case objInfo.Attributes.BashingDamageMax > 0:
			fallthrough
		case objInfo.Attributes.SlashingDamageMax > 0:
			fallthrough
		case objInfo.Attributes.StabbingDamageMax > 0:
			return true
		}
	}
	return false
}

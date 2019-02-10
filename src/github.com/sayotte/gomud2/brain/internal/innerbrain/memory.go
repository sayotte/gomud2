package innerbrain

import (
	"encoding/json"
	"errors"
	"github.com/sayotte/gomud2/commands"
	uuid2 "github.com/sayotte/gomud2/uuid"
	"github.com/sayotte/gomud2/wsapi"
	"sync"
	"time"

	"github.com/satori/go.uuid"
)

type MessageSenderCallbacker interface {
	SendMessage(msg wsapi.Message) error
	RegisterResponseCallback(requestID uuid.UUID, callback func(msg wsapi.Message))
}

func NewMemory(senderCallbacker MessageSenderCallbacker) *Memory {
	return &Memory{
		lock:             &sync.RWMutex{},
		localStore:       make(map[string]json.Marshaler),
		senderCallbacker: senderCallbacker,
	}
}

type Memory struct {
	lock             *sync.RWMutex
	localStore       map[string]json.Marshaler
	senderCallbacker MessageSenderCallbacker
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
		m.senderCallbacker.RegisterResponseCallback(msgID, callback)

		getLocInfoMsg := wsapi.Message{
			Type:      wsapi.MessageTypeGetCurrentLocationInfoCommand,
			MessageID: msgID,
		}
		err = m.senderCallbacker.SendMessage(getLocInfoMsg)
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

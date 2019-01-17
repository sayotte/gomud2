package spawnreap

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/core"
)

const (
	DefaultTickLengthS = 5
	DefaultReapTicks   = 60 // 5 minutes @ 5-second ticks
)

type Service struct {
	World *core.World
	// How often, in seconds, to check for objects that need reaping, and
	// actors that need spawning
	TickLengthS int
	// How many ticks to leave an object in-place before reaping it
	ReapTicks int

	zoneToLocToObjectAgeMap map[uuid.UUID]map[uuid.UUID]map[uuid.UUID]int
	zoneToSpawnSpecsMap     map[uuid.UUID][]SpawnSpecification
	rando                   *rand.Rand

	tickChan chan struct{}
	stopChan chan struct{}
	stopWG   *sync.WaitGroup
}

func (s *Service) Start() error {
	if s.World == nil {
		return errors.New("uninitialized Service.World")
	}

	s.rando = rand.New(rand.NewSource(time.Now().UnixNano()))

	s.zoneToLocToObjectAgeMap = make(map[uuid.UUID]map[uuid.UUID]map[uuid.UUID]int)
	if s.TickLengthS == 0 {
		s.TickLengthS = DefaultTickLengthS
	}
	s.tickChan = make(chan struct{})
	if s.ReapTicks == 0 {
		s.ReapTicks = DefaultReapTicks
	}

	s.stopChan = make(chan struct{})
	go s.mainLoop()
	go s.tickLoop()
	return nil
}

func (s *Service) Stop() {
	s.stopWG = &sync.WaitGroup{}
	s.stopWG.Add(2)
	close(s.stopChan)
	s.stopWG.Wait()
}

func (s *Service) tickLoop() {
	for {
		select {
		case <-s.stopChan:
			s.stopWG.Done()
			return
		default:
		}
		s.tickChan <- struct{}{}
		time.Sleep(time.Duration(s.TickLengthS) * time.Second)
	}
}

func (s *Service) mainLoop() {
	for {
		select {
		case <-s.stopChan:
			s.stopWG.Done()
			return
		case <-s.tickChan:
			s.handleTick()
		}
	}
}

func (s *Service) handleTick() {
	for _, zone := range s.World.Zones() {
		s.reapZone(zone)
	}
}

func (s *Service) reapZone(zone *core.Zone) {
	zoneMap, found := s.zoneToLocToObjectAgeMap[zone.ID()]
	if !found {
		zoneMap = make(map[uuid.UUID]map[uuid.UUID]int)
	}
	for _, loc := range zone.Locations() {
		// only reap objects when Actors aren't around to see it
		var doReap bool
		if len(loc.Actors()) == 0 {
			doReap = true
		}

		locMap, found := zoneMap[loc.ID()]
		if !found {
			locMap = make(map[uuid.UUID]int)
		}
		seenObjects := make(map[uuid.UUID]bool)
		// increment the tick-age of all objects in the map
		for _, object := range loc.Objects() {
			seenObjects[object.ID()] = true
			locMap[object.ID()]++
			if doReap && locMap[object.ID()] > s.ReapTicks {
				err := zone.RemoveObject(object)
				if err != nil {
					fmt.Printf("SpawnReap ERROR: zone.RemoveObject(%s): %s\n", object.ID(), err)
				} else {
					delete(locMap, object.ID())
				}
			}
		}
		// clean out obsolete entries in the map; these are objects that
		// have been removed by game events, so we shouldn't track their
		// age here any more
		for objectID := range locMap {
			if !seenObjects[objectID] {
				delete(locMap, objectID)
			}
		}

		zoneMap[loc.ID()] = locMap
	}
	s.zoneToLocToObjectAgeMap[zone.ID()] = zoneMap
}

func (s *Service) spawnZone(zone *core.Zone) {
	specList, found := s.zoneToSpawnSpecsMap[zone.ID()]
	if !found {
		return
	}

	zoneActors := zone.Actors()
	for _, spec := range specList {
		// determine if we should spawn anything at all
		var currentCount int
		for _, actor := range zoneActors {
			if actor.Name() == spec.ActorProto.Name {
				currentCount++
			}
		}
		if currentCount >= spec.MaxCount {
			continue
		}
		if s.rando.Float64() < spec.SpawnChancePerTick {
			continue
		}

		// determine how many we should spawn
		maxSpawnThisTick := spec.MaxCount - currentCount
		spawnThisTick := s.rando.Intn(maxSpawnThisTick)
		if spawnThisTick == 0 {
			spawnThisTick = 1
		}

		// find a Location to spawn in
		var targetLoc *core.Location
		zoneLocs := zone.Locations()
		for _, loc := range zoneLocs {
			// first try to find a Location with no Actors, to help with immersion
			// note that this includes non-player Actors; this will result in
			// different specs' spawns being distributed around the Zone
			if len(loc.Actors()) == 0 {
				targetLoc = loc
				break
			}
		}
		if targetLoc == nil {
			// failing any empty Locations, just spawn them in a random Location
			idx := s.rando.Intn(len(zoneLocs) - 1)
			targetLoc = zoneLocs[idx]
		}

		// spawn
		for i := 0; i < spawnThisTick; i++ {
			_, err := zone.AddActor(spec.ActorProto.ToActor(targetLoc))
			if err != nil {
				fmt.Printf("SpawnReap ERROR: zone.AddActor(...): %s\n", err)
			}
		}
	}
}

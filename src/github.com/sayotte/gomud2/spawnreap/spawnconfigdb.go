package spawnreap

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/satori/go.uuid"
	"gopkg.in/yaml.v2"

	"github.com/sayotte/gomud2/core"
)

type spawnConfigDatabase struct {
	filename            string
	zoneToSpawnSpecsMap map[uuid.UUID][]SpawnSpecification
	rwlock              *sync.RWMutex
}

func (scdb *spawnConfigDatabase) load() error {
	scdb.rwlock = &sync.RWMutex{}
	scdb.rwlock.Lock()
	defer scdb.rwlock.Unlock()

	scdb.zoneToSpawnSpecsMap = make(map[uuid.UUID][]SpawnSpecification)

	if _, err := os.Stat(scdb.filename); os.IsNotExist(err) {
		return nil
	}

	dbBytes, err := ioutil.ReadFile(scdb.filename)
	if err != nil {
		return fmt.Errorf("ioutil.ReadFile(%q): %s", scdb.filename, err)
	}

	err = yaml.Unmarshal(dbBytes, scdb.zoneToSpawnSpecsMap)
	if err != nil {
		return fmt.Errorf("yaml.Unmarshal(): %s", err)
	}

	return nil
}

func (scdb *spawnConfigDatabase) save() error {
	dbBytes, err := yaml.Marshal(scdb.zoneToSpawnSpecsMap)
	if err != nil {
		return fmt.Errorf("yaml.Marshal(): %s", err)
	}
	err = ioutil.WriteFile(scdb.filename, dbBytes, 0644)
	if err != nil {
		return fmt.Errorf("ioutil.WriteFile(%q): %s", scdb.filename, err)
	}
	return nil
}

func (scdb *spawnConfigDatabase) getEntryForZone(zone *core.Zone) []SpawnSpecification {
	scdb.rwlock.RLock()
	defer scdb.rwlock.RUnlock()
	return scdb.zoneToSpawnSpecsMap[zone.ID()]
}

func (scdb *spawnConfigDatabase) putEntryForZone(specList []SpawnSpecification, zone *core.Zone) error {
	scdb.rwlock.Lock()
	defer scdb.rwlock.Unlock()
	scdb.zoneToSpawnSpecsMap[zone.ID()] = specList
	return scdb.save()
}

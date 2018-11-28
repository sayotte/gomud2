package auth

import (
	"errors"
	"fmt"
	"github.com/satori/go.uuid"
	uuid2 "github.com/sayotte/gomud2/uuid"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"sort"
	"sync"
)

type dbMap map[uuid.UUID]*dbEntry

// MarshalYAML overrides the default behavior which has unstable ordering for
// the keys in the map. This function ensures the keys are always sorted
// alphabetically.
func (dm dbMap) MarshalYAML() (interface{}, error) {
	keys := make([]uuid.UUID, 0, len(dm))
	for key := range dm {
		keys = append(keys, key)
	}
	sort.Sort(uuid2.UUIDList(keys))

	mapItems := make(yaml.MapSlice, 0, len(dm))
	for _, key := range keys {
		mapItems = append(mapItems, yaml.MapItem{
			Key:   key,
			Value: dm[key],
		})
	}
	return mapItems, nil
}

type dbEntry struct {
	AuthZDescriptor AuthZDescriptor
	Username        string
	PasswordHash    string
	Actors          []uuid.UUID
}

type userIndexEntry struct {
	id  uuid.UUID
	ent *dbEntry
}

type authDatabase struct {
	accountDBFile   string
	mapByID         dbMap
	indexByUsername map[string]userIndexEntry
	rwlock          *sync.RWMutex
}

func (adb *authDatabase) load() error {
	adb.mapByID = make(dbMap)
	adb.indexByUsername = make(map[string]userIndexEntry)
	adb.rwlock = &sync.RWMutex{}
	adb.rwlock.Lock()
	defer adb.rwlock.Unlock()

	unmarshallMap := make(map[uuid.UUID]dbEntry)

	if _, err := os.Stat(adb.accountDBFile); os.IsNotExist(err) {
		// if the file doesn't exist, we can just proceed with the
		// empty map; it'll be created later if/when something is saved
		return nil
	}

	dbBytes, err := ioutil.ReadFile(adb.accountDBFile)
	if err != nil {
		return fmt.Errorf("ioutil.ReadFile(%q): %s", adb.accountDBFile, err)
	}

	err = yaml.Unmarshal(dbBytes, unmarshallMap)
	if err != nil {
		return fmt.Errorf("yaml.Unmarshal(): %s", err)
	}

	for id, ent := range unmarshallMap {
		adb.mapByID[id] = &ent
		adb.indexByUsername[ent.Username] = userIndexEntry{
			id:  id,
			ent: &ent,
		}
	}

	return nil
}

func (adb *authDatabase) save() error {
	dbBytes, err := yaml.Marshal(adb.mapByID)
	if err != nil {
		return fmt.Errorf("yaml.Marshal(): %s", err)
	}
	err = ioutil.WriteFile(adb.accountDBFile, dbBytes, 0600)
	if err != nil {
		return fmt.Errorf("ioutil.WriteFile(%q): %s", adb.accountDBFile, err)
	}
	return nil
}

func (adb *authDatabase) getEntryByID(id uuid.UUID) (dbEntry, error) {
	adb.rwlock.RLock()
	defer adb.rwlock.RUnlock()

	var nilEntry dbEntry
	ent, found := adb.mapByID[id]
	if !found {
		return nilEntry, errors.New("no such user in database")
	}
	return *ent, nil
}

func (adb *authDatabase) getEntryByUsername(username string) (uuid.UUID, dbEntry, error) {
	adb.rwlock.RLock()
	defer adb.rwlock.RUnlock()

	var nilEntry dbEntry
	indexEnt, found := adb.indexByUsername[username]
	if !found {
		return uuid.Nil, nilEntry, errors.New("no such user in database")
	}
	return indexEnt.id, *indexEnt.ent, nil
}

func (adb *authDatabase) putEntry(id uuid.UUID, ent dbEntry) error {
	adb.rwlock.Lock()
	defer adb.rwlock.Unlock()
	newEnt := ent
	adb.mapByID[id] = &newEnt
	adb.indexByUsername[ent.Username] = userIndexEntry{
		id:  id,
		ent: &newEnt,
	}
	return adb.save()
}

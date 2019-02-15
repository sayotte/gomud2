package core

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/rpc"
)

const (
	// FIXME this braintype should be specified via config file, not a const
	// FIXME once that's done, be sure to delete this const to find all places
	// FIXME that need to use the configurable value instead
	PlayerParkingBrainType = "player-parking"
)

type IntentLogger interface {
	Open(func(redo, undo []Event) error) error
	Close()
	WriteIntent(redo, undo []Event) (uuid.UUID, error)
	ConfirmIntentCompletion(transactionID uuid.UUID) error
}

func NewWorld() *World {
	return &World{
		zonesByID: make(map[uuid.UUID]*Zone),
	}
}

type World struct {
	DataStore DataStore
	IntentLog IntentLogger

	frontDoorZone     *Zone
	frontDoorLocation *Location

	zones       []*Zone
	zonesByID   map[uuid.UUID]*Zone
	started     bool
	stopChan    chan struct{}
	stopWG      *sync.WaitGroup
	commandChan chan rpc.Request
}

func (w *World) LoadAndStart(zoneTags []string, defaultZoneID, defaultLocID uuid.UUID) error {
	if w.DataStore == nil {
		return errors.New("World.DataStore must be non-nil")
	}
	if w.IntentLog == nil {
		return errors.New("World.IntentLog must be non-nil")
	}

	err := w.start()
	if err != nil {
		return err
	}

	for _, zoneTag := range zoneTags {
		err = w.LoadZone(zoneTag)
		if err != nil {
			return err
		}
	}

	if !uuid.Equal(defaultZoneID, uuid.Nil) {
		defaultZone := w.ZoneByID(defaultZoneID)
		if defaultZone == nil {
			return fmt.Errorf("default Zone %q not found in World", defaultZoneID)
		}
		defaultLoc := defaultZone.LocationByID(defaultLocID)
		if defaultLoc == nil {
			return fmt.Errorf("default Location %q not found in default Zone", defaultLocID)
		}
		w.frontDoorZone = defaultZone
		w.frontDoorLocation = defaultLoc
	} else {
		fmt.Println("CORE WARNING: starting World with no default Zone/Location; this makes sense in a debugging context, but otherwise is surely an error")
	}

	err = w.ReplayIntentLog()
	if err != nil {
		return err
	}

	return nil
}

// LoadZone creates a Zone with the given tag, finds all events in the
// datastore associated with that tag and replays them into the Zone, and then
// adds the Zone to the World in a runtime-safe fashion (i.e. this can be
// called while the World is live).
func (w *World) LoadZone(zoneTag string) error {
	tagParts := strings.Split(zoneTag, "/")
	zoneID, err := uuid.FromString(tagParts[1])
	if err != nil {
		return fmt.Errorf("uuid.FromString(%q): %s", tagParts[1], err)
	}
	z := NewZone(zoneID, tagParts[0], nil)

	eChan, err := w.DataStore.RetrieveAllEventsForZone(zoneID)
	if err != nil {
		return err
	}
	err = z.ReplayEvents(eChan)
	if err != nil {
		return err
	}

	z.setPersister(w.DataStore)

	z.StartCommandProcessing()

	err = w.AddZone(z)
	if err != nil {
		return err
	}

	return nil
}

func (w *World) start() error {
	if w.started {
		return errors.New("World already started")
	}

	w.stopChan = make(chan struct{})
	w.stopWG = &sync.WaitGroup{}
	w.commandChan = make(chan rpc.Request)

	w.started = true
	go w.processCommandsLoop()
	return nil
}

func (w *World) handleIncompleteTransactions(redo, undo []Event) error {
	for _, e := range undo {
		if e.Type() != EventTypeActorAddToZone {
			return fmt.Errorf("unhandleable World undo-event type %T", e)
		}
		readdEvent := e.(ActorAddToZoneEvent)
		oldZone, found := w.zonesByID[readdEvent.AggregateID]
		if !found {
			return fmt.Errorf("no such Zone with ID %q", readdEvent.AggregateID)
		}
		oldLoc := oldZone.LocationByID(readdEvent.StartingLocationID)
		if oldLoc == nil {
			return fmt.Errorf("no such Location with ID %q", readdEvent.StartingLocationID)
		}

		actor := NewActor(
			readdEvent.ActorID,
			readdEvent.Name,
			readdEvent.BrainType,
			oldLoc,
			oldZone,
			readdEvent.Attributes,
			readdEvent.Skills,
			readdEvent.InventoryConstraints,
		)
		_, err := oldZone.AddActor(actor)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *World) processCommandsLoop() {
	for {
		select {
		case <-w.stopChan:
			w.stopWG.Done()
			return
		case req := <-w.commandChan:
			var res rpc.Response
			switch req.Payload.(type) {
			case worldAddActorCommand:
				waac := req.Payload.(worldAddActorCommand)
				res.Value, res.Err = w.handleAddActor(waac.actor)
			case worldMigrateActorCommand:
				wmac := req.Payload.(worldMigrateActorCommand)
				res.Value, res.Err = w.handleMigrateActorCommand(wmac)
			case worldAddZoneCommand:
				wazc := req.Payload.(worldAddZoneCommand)
				res.Err = w.handleAddZone(wazc.zone)
			case worldSnapshotCommand:
				res.Err = w.handleSnapshot()
			case worldReplayIntentLogCommand:
				incompleteTransactionHandler := func(redo, undo []Event) error {
					return w.handleIncompleteTransactions(redo, undo)
				}
				res.Err = w.IntentLog.Open(incompleteTransactionHandler)
			default:
				res.Err = fmt.Errorf("unknown World command type %T", req.Payload)
			}
			req.ResponseChan <- res
		}
	}
}

func (w *World) Stop() {
	w.stopWG.Add(1)
	close(w.stopChan)
	w.stopWG.Wait()
	w.IntentLog.Close()
}

func (w *World) syncRequestToSelf(payload interface{}) (interface{}, error) {
	req := rpc.Request{
		Payload:      payload,
		ResponseChan: make(chan rpc.Response),
	}
	w.commandChan <- req
	response := <-req.ResponseChan
	return response.Value, response.Err
}

func (w *World) AddActor(a *Actor) (*Actor, error) {
	out, err := w.syncRequestToSelf(worldAddActorCommand{actor: a})
	if err != nil {
		return nil, err
	}
	return out.(*Actor), nil
}

func (w *World) handleAddActor(a *Actor) (*Actor, error) {
	a.setLocation(w.frontDoorLocation)
	a.setZone(w.frontDoorZone)
	return w.frontDoorZone.AddActor(a)
}

func (w *World) MigrateActor(a *Actor, fromLoc, toLoc *Location) (*Actor, error) {
	if fromLoc.Zone() == toLoc.Zone() {
		return nil, errors.New("World.MigrateActor() doesn't make sense within the same Zone; use Actor.Move()")
	}

	cmd := worldMigrateActorCommand{
		actor:   a,
		fromLoc: fromLoc,
		toLoc:   toLoc,
	}
	out, err := w.syncRequestToSelf(cmd)
	if err != nil {
		return nil, err
	}
	return out.(*Actor), nil
}

func (w *World) handleMigrateActorCommand(cmd worldMigrateActorCommand) (*Actor, error) {
	//undoActorRemove := a.snapshot(0)
	//transactionID, err := w.IntentLog.WriteIntent(nil, []Event{undoActorRemove})
	//if err != nil {
	//	return nil, err
	//}

	newActor, err := cmd.toLoc.Zone().MigrateInActor(
		cmd.actor,
		cmd.fromLoc,
		cmd.toLoc,
	)
	if err != nil {
		return nil, err
	}
	err = cmd.fromLoc.Zone().MigrateOutActor(
		cmd.actor,
		cmd.fromLoc,
		cmd.toLoc,
	)
	if err != nil {
		return nil, err
	}

	//err = w.IntentLog.ConfirmIntentCompletion(transactionID)
	//if err != nil {
	//	return newActor, err
	//}

	return newActor, nil
}

func (w *World) AddZone(z *Zone) error {
	_, err := w.syncRequestToSelf(worldAddZoneCommand{zone: z})
	return err
}

func (w *World) handleAddZone(z *Zone) error {
	_, dupe := w.zonesByID[z.ID()]
	if dupe {
		return errors.New("zone already present in World")
	}
	w.zones = append(w.zones, z)
	w.zonesByID[z.ID()] = z
	z.setWorld(w)
	return nil
}

func (w *World) ZoneByID(id uuid.UUID) *Zone {
	return w.zonesByID[id]
}

func (w *World) Zones() []*Zone {
	out := make([]*Zone, len(w.zones))
	copy(out, w.zones)
	return out
}

func (w *World) ActorByID(id uuid.UUID) *Actor {
	for _, zone := range w.zones {
		a := zone.ActorByID(id)
		if a != nil {
			return a
		}
	}
	return nil
}

func (w *World) Snapshot() error {
	_, err := w.syncRequestToSelf(worldSnapshotCommand{})
	return err
}

func (w *World) handleSnapshot() error {
	// stop all event processing so we're sure to get sequence numbers
	// representing a single instant in time across all zones
	for _, zone := range w.zones {
		zone.StopCommandProcessing()
	}
	zoneIDToSeqNum := make(map[uuid.UUID]uint64)
	zoneIDToNickname := make(map[uuid.UUID]string)
	for _, zone := range w.zones {
		zoneIDToSeqNum[zone.ID()] = zone.LastSequenceNum()
		zoneIDToNickname[zone.ID()] = zone.Nickname()
		// restart processing events for each zone after getting its current
		// sequence number
		zone.StartCommandProcessing()
	}

	for zoneId, seqNum := range zoneIDToSeqNum {
		zone := NewZone(zoneId, zoneIDToNickname[zoneId], nil)
		eChan, err := w.DataStore.RetrieveEventsUpToSequenceNumForZone(seqNum, zoneId)
		if err != nil {
			return err
		}
		err = zone.ReplayEvents(eChan)
		if err != nil {
			return err
		}

		snapEvents := zone.snapshot(seqNum)
		err = w.DataStore.PersistSnapshot(zoneId, seqNum, snapEvents)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *World) ReplayIntentLog() error {
	_, err := w.syncRequestToSelf(worldReplayIntentLogCommand{})
	return err
}

type worldReplayIntentLogCommand struct{}

type worldAddActorCommand struct {
	actor *Actor
}

type worldMigrateActorCommand struct {
	actor          *Actor
	fromLoc, toLoc *Location
}

type worldAddZoneCommand struct {
	zone *Zone
}

type worldSnapshotCommand struct{}

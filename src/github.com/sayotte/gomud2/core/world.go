package core

import (
	"errors"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/rpc"
	"sync"
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

func (w *World) LoadAndStart(zoneIDs []uuid.UUID, defaultZoneID, defaultLocID uuid.UUID) error {
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

	for _, zoneID := range zoneIDs {
		z := NewZone(nil)
		z.Id = zoneID
		err := w.AddZone(z)
		if err != nil {
			return err
		}

		eChan, err := w.DataStore.RetrieveAllForZone(zoneID)
		if err != nil {
			return err
		}
		err = z.ReplayEvents(eChan)
		if err != nil {
			return err
		}

		z.SetPersister(w.DataStore)
	}

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

	for _, zone := range w.Zones() {
		zone.StartEventProcessing()
	}

	err = w.ReplayIntentLog()
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
		oldZone, found := w.zonesByID[readdEvent.aggregateId]
		if !found {
			return fmt.Errorf("no such Zone with ID %q", readdEvent.aggregateId)
		}
		oldLoc := oldZone.LocationByID(readdEvent.startingLocationId)
		if oldLoc == nil {
			return fmt.Errorf("no such Location with ID %q", readdEvent.startingLocationId)
		}

		actor := NewActor(readdEvent.Name(), oldLoc, oldZone)
		actor.Id = readdEvent.actorId
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
				res.Value, res.Err = w.handleMigrateActor(
					wmac.actor,
					wmac.fromZone,
					wmac.toZone,
					wmac.toLoc,
					wmac.observer,
				)
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
	a.location = w.frontDoorLocation
	a.zone = w.frontDoorZone
	return w.frontDoorZone.AddActor(a)
}

func (w *World) MigrateActor(a *Actor, fromZone *Zone, toZoneID, toLocID uuid.UUID, o Observer) (*Actor, error) {
	toZone, found := w.zonesByID[toZoneID]
	if !found {
		return nil, fmt.Errorf("no such Zone %q", toZoneID)
	}
	toLoc, found := toZone.locationsById[toLocID]
	if !found {
		return nil, fmt.Errorf("no such Location %q", toLocID)
	}
	cmd := worldMigrateActorCommand{
		actor:    a,
		fromZone: fromZone,
		toZone:   toZone,
		toLoc:    toLoc,
		observer: o,
	}
	out, err := w.syncRequestToSelf(cmd)
	if err != nil {
		return nil, err
	}
	return out.(*Actor), nil
}

func (w *World) handleMigrateActor(a *Actor, fromZone, toZone *Zone, toLoc *Location, o Observer) (*Actor, error) {
	undoActorRemove := a.snapshot(0)
	transactionID, err := w.IntentLog.WriteIntent(nil, []Event{undoActorRemove})
	if err != nil {
		return nil, err
	}

	err = fromZone.RemoveActor(a)
	if err != nil {
		return nil, err
	}
	a.location = toLoc
	a.zone = toZone
	newActor, err := toZone.MigrateInActor(a, o)
	if err != nil {
		return nil, err
	}
	err = w.IntentLog.ConfirmIntentCompletion(transactionID)
	if err != nil {
		return newActor, err
	}
	return newActor, nil
}

func (w *World) AddZone(z *Zone) error {
	_, err := w.syncRequestToSelf(worldAddZoneCommand{zone: z})
	return err
}

func (w *World) handleAddZone(z *Zone) error {
	_, dupe := w.zonesByID[z.Id]
	if dupe {
		return errors.New("zone already present in World")
	}
	w.zones = append(w.zones, z)
	w.zonesByID[z.Id] = z
	z.setWorld(w)
	return nil
}

func (w *World) ZoneByID(id uuid.UUID) *Zone {
	return w.zonesByID[id]
}

func (w *World) Zones() []*Zone {
	return w.zones
}

func (w *World) ActorByID(id uuid.UUID) *Actor {
	for _, zone := range w.zones {
		a, found := zone.actorsById[id]
		if found {
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
		zone.StopEventProcessing()
	}
	zoneIDToSeqNum := make(map[uuid.UUID]uint64)
	for _, zone := range w.zones {
		zoneIDToSeqNum[zone.Id] = zone.nextSequenceId - 1
		// restart processing events for each zone after getting its current
		// sequence number
		zone.StartEventProcessing()
	}

	for zoneId, seqNum := range zoneIDToSeqNum {
		zone := NewZone(nil)
		zone.Id = zoneId
		eChan, err := w.DataStore.RetrieveUpToSequenceNumsForZone(seqNum, zoneId)
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
	actor            *Actor
	fromZone, toZone *Zone
	toLoc            *Location
	observer         Observer
}

type worldAddZoneCommand struct {
	zone *Zone
}

type worldSnapshotCommand struct{}

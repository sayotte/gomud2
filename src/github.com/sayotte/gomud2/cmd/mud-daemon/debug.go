package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/abiosoft/ishell"
	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/core"
	"github.com/sayotte/gomud2/rpc"
	"github.com/sayotte/gomud2/store"
	"github.com/sayotte/gomud2/telnet"
)

const timestampLayout = "02 Jan 2006 15:04:05.000"

var eventTypeToStringName = map[int]string{
	core.EventTypeActorMove:              "ActorMoveEvent",
	core.EventTypeActorAdminRelocate:     "ActorAdminRelocateEvent",
	core.EventTypeActorAddToZone:         "ActorAddToZoneEvent",
	core.EventTypeActorRemoveFromZone:    "ActorRemoveFromZoneEvent",
	core.EventTypeActorDeath:             "ActorDeathEvent",
	core.EventTypeActorMigrateIn:         "ActorMigrateInEvent",
	core.EventTypeActorMigrateOut:        "ActorMigrateOutEvent",
	core.EventTypeActorSpeak:             "ActorSpeakEvent",
	core.EventTypeLocationAddToZone:      "LocationAddToZoneEvent",
	core.EventTypeLocationRemoveFromZone: "LocationRemoveFromZoneEvent",
	core.EventTypeLocationUpdate:         "LocationUpdateEvent",
	core.EventTypeExitAddToZone:          "ExitAddToZoneEvent",
	core.EventTypeExitUpdate:             "ExitUpdateEvent",
	core.EventTypeExitRemoveFromZone:     "ExitRemoveFromZoneEvent",
	core.EventTypeObjectAddToZone:        "ObjectAddToZoneEvent",
	core.EventTypeObjectRemoveFromZone:   "ObjectRemoveFromZoneEvent",
	core.EventTypeObjectMove:             "ObjectMoveEvent",
	core.EventTypeObjectMoveSubcontainer: "ObjectMoveSubcontainerEvent",
	core.EventTypeObjectAdminRelocate:    "ObjectAdminRelocateEvent",
	core.EventTypeObjectMigrateIn:        "ObjectMigrateInEvent",
	core.EventTypeObjectMigrateOut:       "ObjectMigrateOutEvent",
	core.EventTypeZoneSetDefaultLocation: "ZoneSetDefaultLocationEvent",
	core.EventTypeCombatMeleeDamage:      "CombatMeleeDamageEvent",
}

type debugger struct {
	mudConfig *mudConfig
	world     *core.World
	zone      *core.Zone

	authService  *auth.Server
	telnetServer *telnet.Server

	dataStore      core.DataStore
	inEventStream  <-chan rpc.Response
	outEventStream chan rpc.Response
	zoneStarted    bool

	breakpoints  []breakpoint
	currentEvent core.Event

	shell *ishell.Shell
}

func (d *debugger) init(cfg *mudConfig, datastore core.DataStore) {
	d.mudConfig = cfg
	d.dataStore = datastore

	d.shell = ishell.New()

	// break
	breakCmd := &ishell.Cmd{
		Name: "break",
		Help: "create a new breakpoint",
		Func: d.getBreakHandler(),
	}
	breakCmd.LongHelp = "Syntax: break <time|actor|exit|location|object> <subcmd args>\n\n"
	breakCmd.LongHelp += "Creates a breakpoint, which will cause the debugger to stop replaying events\n"
	breakCmd.LongHelp += "when the conditions of the breakpoint are matched. This allows the user to\n"
	breakCmd.LongHelp += "reach a certain point in the event-stream, and then inspect the current state of\n"
	breakCmd.LongHelp += "the Zone."
	breakTimeCmd := &ishell.Cmd{
		Name: "time",
		Help: "<timestamp> - break on events after a given timestamp",
		Func: d.getBreakTimeHandler(),
	}
	breakCmd.AddCmd(breakTimeCmd)
	breakActorCmd := &ishell.Cmd{
		Name: "actor",
		Help: "<UUID> - break on events involving the given Actor",
		Func: d.getBreakActorHandler(),
	}
	breakCmd.AddCmd(breakActorCmd)
	breakExitCmd := &ishell.Cmd{
		Name: "exit",
		Help: "<UUID> - break on events involving the given Exit",
		Func: d.getBreakExitHandler(),
	}
	breakCmd.AddCmd(breakExitCmd)
	breakLocCmd := &ishell.Cmd{
		Name: "location",
		Help: "<UUID> - break on events involving the given Location",
		Func: d.getBreakLocationHandler(),
	}
	breakCmd.AddCmd(breakLocCmd)
	breakObjCmd := &ishell.Cmd{
		Name: "object",
		Help: "<UUID> - break on events involving the given Object",
		Func: d.getBreakObjectHandler(),
	}
	breakCmd.AddCmd(breakObjCmd)
	d.shell.AddCmd(breakCmd)

	// breakpoints
	d.shell.AddCmd(&ishell.Cmd{
		Name: "breakpoints",
		Help: "list currently-set breakpoints",
		Func: d.getBreakpointsHandler(),
	})
	d.shell.AddCmd(&ishell.Cmd{
		Name: "bp",
		Help: "alias for breakpoints",
		Func: d.getBreakpointsHandler(),
	})

	// clear
	d.shell.DeleteCmd("clear")
	d.shell.AddCmd(&ishell.Cmd{
		Name: "clear",
		Help: "clear a breakpoint",
		Func: d.getClearHandler(),
	})

	// continue
	d.shell.AddCmd(&ishell.Cmd{
		Name: "continue",
		Help: "replay events until next breakpoint is hit",
		Func: d.getContinueHandler(),
	})
	d.shell.AddCmd(&ishell.Cmd{
		Name: "c",
		Help: "alias for continue",
		Func: d.getContinueHandler(),
	})

	// next
	d.shell.AddCmd(&ishell.Cmd{
		Name:     "next",
		Help:     "replays one event",
		LongHelp: "LongHelp displayed here!",
		Func:     d.getNextHandler(),
	})
	d.shell.AddCmd(&ishell.Cmd{
		Name: "n",
		Help: "alias for next",
		Func: d.getNextHandler(),
	})

	// shownext
	d.shell.AddCmd(&ishell.Cmd{
		Name: "shownext",
		Help: "show next event to be replayed",
		Func: d.getShownextHandler(),
	})
	d.shell.AddCmd(&ishell.Cmd{
		Name: "l",
		Help: "alias for shownext",
		Func: d.getShownextHandler(),
	})

	// starttelnet
	d.shell.AddCmd(&ishell.Cmd{
		Name: "starttelnet",
		Help: "start Telnet server (PROCEED CAREFULLY!)",
		Func: d.getStartTelnetHandler(),
	})

	// stoptelnet
	d.shell.AddCmd(&ishell.Cmd{
		Name: "stoptelnet",
		Help: "stop Telnet server",
		Func: d.getStopTelnetHandler(),
	})

	// zone
	d.shell.AddCmd(&ishell.Cmd{
		Name: "zone",
		Help: "debug the given Zone (discards currently debugged Zone)",
		Func: d.getZoneHandler(),
	})
}

func (d *debugger) run() {
	d.shell.Run()
}

func (d *debugger) showEvent(e core.Event, c *ishell.Context) {
	outBytes, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		c.Println("Error rendering Event to JSON: %s\n", err)
	}
	c.Printf("%s %s\n", eventTypeToStringName[e.Type()], string(outBytes))
}

func (d *debugger) incrementStream(c *ishell.Context) {
	if d.currentEvent != nil {
		outResponse := rpc.Response{
			Value: d.currentEvent,
		}
		d.outEventStream <- outResponse
	}
	nextResponse, notClosed := <-d.inEventStream
	if !notClosed {
		c.Println("No more events in stream!")
		d.currentEvent = nil
		return
	}
	if nextResponse.Err != nil {
		c.Printf("Response from event-stream is an error (not an Event): %s\n", nextResponse.Err)
		d.currentEvent = nil
		return
	}
	var ok bool
	d.currentEvent, ok = nextResponse.Value.(core.Event)
	if !ok {
		c.Printf("Response from event-stream is not an Event, but also not an error? %v\n", nextResponse.Value)
	}
}

func (d *debugger) getBreakHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		c.Println("break?")
	}
}

func (d *debugger) getBreakTimeHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		if len(c.Args) != 1 {
			c.Println("Invalid syntax, need: break time <timestamp>, where <timestamp> is of the form %q (don't forget to use quotes!).\n", timestampLayout)
			return
		}
		t, err := time.Parse(timestampLayout, c.Args[0])
		if err != nil {
			c.Printf("Can't parse that timestamp: %s\n", err)
			return
		}

		bp := timeBreakpoint{
			timestamp: t,
		}
		d.breakpoints = append(d.breakpoints, bp)
	}
}

func (d *debugger) getBreakActorHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		if len(c.Args) != 1 {
			c.Println("Invalid syntax, need: break actor <UUID>, where <UUID> is the ID of the Actor\n")
			return
		}
		actorID, err := uuid.FromString(c.Args[0])
		if err != nil {
			c.Printf("Invalid UUID: %s\n", err)
			return
		}

		bp := actorBreakpoint{
			actorID: actorID,
		}
		d.breakpoints = append(d.breakpoints, bp)
	}
}

func (d *debugger) getBreakExitHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		if len(c.Args) != 1 {
			c.Println("Invalid syntax, need: break exit <UUID>, where <UUID> is the ID of the Exit\n")
			return
		}
		exitID, err := uuid.FromString(c.Args[0])
		if err != nil {
			c.Printf("Invalid UUID: %s\n", err)
			return
		}

		bp := exitBreakpoint{
			exitID: exitID,
		}
		d.breakpoints = append(d.breakpoints, bp)
	}
}

func (d *debugger) getBreakLocationHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		if len(c.Args) != 1 {
			c.Println("Invalid syntax, need: break location <UUID>, where <UUID> is the ID of the Location\n")
			return
		}
		locID, err := uuid.FromString(c.Args[0])
		if err != nil {
			c.Printf("Invalid UUID: %s\n", err)
			return
		}

		bp := locBreakpoint{
			locID: locID,
		}
		d.breakpoints = append(d.breakpoints, bp)
	}
}

func (d *debugger) getBreakObjectHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		if len(c.Args) != 1 {
			c.Println("Invalid syntax, need: break object <UUID>, where <UUID> is the ID of the Object\n")
			return
		}
		objID, err := uuid.FromString(c.Args[0])
		if err != nil {
			c.Printf("Invalid UUID: %s\n", err)
			return
		}

		bp := objBreakpoint{
			objectID: objID,
		}
		d.breakpoints = append(d.breakpoints, bp)
	}
}

func (d *debugger) getBreakpointsHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		for i := 0; i < len(d.breakpoints); i++ {
			c.Printf("[%d] %s\n", i, d.breakpoints[i])
		}
	}
}

func (d *debugger) getClearHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		if len(c.Args) != 1 {
			c.Println("Invalid syntax, need: clear <number>, where <number> is the index of the breakpoint to be cleared in the output of 'breakpoints'\n")
			return
		}
		idx, err := strconv.Atoi(c.Args[0])
		if err != nil {
			c.Printf("Expected an integer, got %q. Try again?\n", c.Args[0])
			return
		}
		if idx >= len(d.breakpoints) {
			c.Println("No such breakpoint.")
			return
		}
		d.breakpoints = append(d.breakpoints[:idx], d.breakpoints[idx+1:]...)
	}
}

func (d *debugger) getContinueHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		for {
			d.incrementStream(c)
			if d.currentEvent == nil {
				return
			}
			for _, bp := range d.breakpoints {
				if bp.shouldBreak(d.currentEvent) {
					d.showEvent(d.currentEvent, c)
					return
				}
			}
		}
	}
}

func (d *debugger) getNextHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		if d.inEventStream == nil {
			c.Println("No Zone loaded, use the 'zone' command first.")
			return
		}
		d.incrementStream(c)
		if d.currentEvent != nil {
			d.showEvent(d.currentEvent, c)
		}
	}
}

func (d *debugger) getShownextHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		if d.currentEvent == nil {
			c.Println("No next event to show.")
			return
		}
		d.showEvent(d.currentEvent, c)
	}
}

func (d *debugger) getStartTelnetHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		if d.mudConfig == nil {
			c.Println("Weird, we weren't passed a mudConfig object, can't proceed!")
			return
		}

		if d.world == nil {
			c.Println("No Zone loaded, use the 'zone' command to load one before starting Telnet.")
			return
		}

		if d.telnetServer != nil {
			c.Println("Telnet server already started!")
			return
		}

		if d.authService == nil {
			authService := &auth.Server{
				AccountDatabaseFile: "auth.db",
			}
			err := authService.Start()
			if err != nil {
				c.Printf("Error starting Auth service, aborting: %s\n", err)
				os.Exit(1)
			}
			d.authService = authService
		}

		telnetServer := &telnet.Server{
			ListenAddr:      d.mudConfig.Telnet.ListenAddr,
			MessageQueueLen: telnet.DefaultMessageQueueLen,
			AuthService:     d.authService,
			World:           d.world,
		}
		err := telnetServer.Start()
		if err != nil {
			c.Printf("Error starting Telnet server, aborting: %s\n", err)
			os.Exit(1)
		}
		d.telnetServer = telnetServer

		c.Printf("Telnet server running on %s, PROCEED WITH CAUTION.\n", d.mudConfig.Telnet.ListenAddr)
	}
}

func (d *debugger) stopTelnetServer(c *ishell.Context) {
	if d.telnetServer == nil {
		c.Println("Telnet server not started.")
		return
	}

	err := d.telnetServer.Stop()
	if err != nil {
		c.Printf("Error stopping Telnet server, aborting: %s\n", err)
		os.Exit(1)
	}

	d.telnetServer = nil

	c.Println("Telnet server stopped.")
}

func (d *debugger) getStopTelnetHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		d.stopTelnetServer(c)
	}
}

func (d *debugger) getZoneHandler() func(c *ishell.Context) {
	return func(c *ishell.Context) {
		if d.mudConfig == nil {
			c.Println("Weird, we weren't passed a mudConfig object, aborting!")
			os.Exit(1)
		}

		if len(c.Args) != 1 {
			c.Println("Invalid syntax, need: zone <tag>, where <tag> is \"nickname/UUID\"")
			return
		}
		tagParts := strings.Split(c.Args[0], "/")
		zoneID, err := uuid.FromString(tagParts[1])
		if err != nil {
			c.Printf("Invalid Zone UUID %q: %s\n", tagParts[1], err)
			return
		}

		// Make sure to stop and discard any previously-debugged World/Zone,
		// so that the garbage collector can grab them
		if d.zone != nil {
			if d.zoneStarted {
				d.zone.StopCommandProcessing()
			}
			if d.telnetServer != nil {
				d.stopTelnetServer(c)
			}
			close(d.outEventStream)
			d.outEventStream = nil
			for range d.inEventStream {
				// do nothing, we're just draining the channel before deleting it
			}
			d.inEventStream = nil
			d.currentEvent = nil
			d.zone = nil

			d.world.Stop()
			d.world = nil
		}

		zone := core.NewZone(zoneID, tagParts[0], nil)

		world := core.NewWorld()
		world.DataStore = d.dataStore
		world.IntentLog = &store.IntentLogger{
			Filename: d.mudConfig.Store.IntentLogfile,
		}
		err = world.LoadAndStart(nil, uuid.Nil, uuid.Nil)
		if err != nil {
			c.Printf("Error starting World, aborting!: %s\n", err)
			os.Exit(1)
		}
		err = world.AddZone(zone)
		if err != nil {
			c.Printf("Error adding Zone to World, aborting!: %s\n", err)
			os.Exit(1)
		}

		d.outEventStream = make(chan rpc.Response)
		go func() {
			err := zone.ReplayEvents(d.outEventStream)
			if err != nil {
				c.Printf("DEBUGGER ERROR: while replaying event stream, must abort: %s\n", err)
				os.Exit(0)
			}
		}()

		d.inEventStream, err = d.dataStore.RetrieveAllEventsForZone(zoneID)
		if err != nil {
			c.Printf("DEBUGGER ERROR: while retrieving events for Zone, must abort: %s\n", err)
			os.Exit(0)
		}

		d.world = world
		d.zone = zone

		c.Println("World and Zone loaded, no events replayed... ready to debug.")
	}
}

type breakpoint interface {
	shouldBreak(e core.Event) bool
	fmt.Stringer
}

type actorBreakpoint struct {
	actorID uuid.UUID
}

func (ab actorBreakpoint) shouldBreak(e core.Event) bool {
	switch e.Type() {
	case core.EventTypeActorAddToZone:
		typed := e.(*core.ActorAddToZoneEvent)
		return uuid.Equal(typed.ActorID, ab.actorID)
	case core.EventTypeActorRemoveFromZone:
		typed := e.(*core.ActorRemoveFromZoneEvent)
		return uuid.Equal(typed.ActorID, ab.actorID)
	case core.EventTypeActorMove:
		typed := e.(*core.ActorMoveEvent)
		_, _, actorID := typed.FromToActorIDs()
		return uuid.Equal(actorID, ab.actorID)
	case core.EventTypeActorAdminRelocate:
		typed := e.(*core.ActorAdminRelocateEvent)
		return uuid.Equal(typed.ActorID, ab.actorID)
	case core.EventTypeActorMigrateIn:
		typed := e.(*core.ActorMigrateInEvent)
		return uuid.Equal(typed.ActorID, ab.actorID)
	case core.EventTypeActorMigrateOut:
		typed := e.(*core.ActorMigrateOutEvent)
		return uuid.Equal(typed.ActorID, ab.actorID)
	case core.EventTypeObjectMove:
		typed := e.(*core.ObjectMoveEvent)
		if uuid.Equal(typed.ActorID, ab.actorID) {
			return true
		}
		if uuid.Equal(typed.ToActorContainerID, ab.actorID) {
			return true
		}
		if uuid.Equal(typed.FromActorContainerID, ab.actorID) {
			return true
		}
		return false
	default:
		return false
	}
}

func (ab actorBreakpoint) String() string {
	return fmt.Sprintf("break actor %s", ab.actorID)
}

type exitBreakpoint struct {
	exitID uuid.UUID
}

func (eb exitBreakpoint) shouldBreak(e core.Event) bool {
	switch e.Type() {
	case core.EventTypeExitAddToZone:
		typed := e.(*core.ExitAddToZoneEvent)
		return uuid.Equal(typed.ExitID, eb.exitID)
	case core.EventTypeExitUpdate:
		typed := e.(*core.ExitUpdateEvent)
		return uuid.Equal(typed.ExitID, eb.exitID)
	case core.EventTypeExitRemoveFromZone:
		typed := e.(*core.ExitRemoveFromZoneEvent)
		return uuid.Equal(typed.ExitID, eb.exitID)
	default:
		return false
	}
}

func (eb exitBreakpoint) String() string {
	return fmt.Sprintf("break exit %s", eb.exitID)
}

type locBreakpoint struct {
	locID uuid.UUID
}

func (lb locBreakpoint) shouldBreak(e core.Event) bool {
	switch e.Type() {
	case core.EventTypeActorMove:
		typed := e.(*core.ActorMoveEvent)
		if uuid.Equal(typed.FromLocationId, lb.locID) {
			return true
		}
		if uuid.Equal(typed.ToLocationId, lb.locID) {
			return true
		}
		return false
	case core.EventTypeActorAdminRelocate:
		typed := e.(*core.ActorAdminRelocateEvent)
		return uuid.Equal(typed.ToLocationID, lb.locID)
	case core.EventTypeActorAddToZone:
		typed := e.(*core.ActorAddToZoneEvent)
		return uuid.Equal(typed.StartingLocationID, lb.locID)
	case core.EventTypeActorMigrateIn:
		typed := e.(*core.ActorMigrateInEvent)
		return uuid.Equal(typed.ToLocID, lb.locID)
	case core.EventTypeActorMigrateOut:
		typed := e.(*core.ActorMigrateOutEvent)
		return uuid.Equal(typed.FromLocID, lb.locID)
	case core.EventTypeLocationAddToZone:
		typed := e.(*core.LocationAddToZoneEvent)
		return uuid.Equal(typed.LocationID, lb.locID)
	case core.EventTypeLocationRemoveFromZone:
		typed := e.(*core.LocationRemoveFromZoneEvent)
		return uuid.Equal(typed.LocationID, lb.locID)
	case core.EventTypeLocationUpdate:
		typed := e.(*core.LocationUpdateEvent)
		return uuid.Equal(typed.LocationID, lb.locID)
	case core.EventTypeExitAddToZone:
		typed := e.(*core.ExitAddToZoneEvent)
		if uuid.Equal(typed.SourceLocationId, lb.locID) {
			return true
		}
		if uuid.Equal(typed.DestLocationId, lb.locID) {
			return true
		}
		return false
	case core.EventTypeObjectAddToZone:
		typed := e.(*core.ObjectAddToZoneEvent)
		return uuid.Equal(typed.LocationContainerID, lb.locID)
	case core.EventTypeObjectMove:
		typed := e.(*core.ObjectMoveEvent)
		if uuid.Equal(typed.FromLocationContainerID, lb.locID) {
			return true
		}
		if uuid.Equal(typed.ToLocationContainerID, lb.locID) {
			return true
		}
		return false
	case core.EventTypeObjectAdminRelocate:
		typed := e.(*core.ObjectAdminRelocateEvent)
		return uuid.Equal(typed.ToLocationContainerID, lb.locID)
	case core.EventTypeZoneSetDefaultLocation:
		typed := e.(*core.ZoneSetDefaultLocationEvent)
		return uuid.Equal(typed.LocationID, lb.locID)
	default:
		return false
	}
}

func (lb locBreakpoint) String() string {
	return fmt.Sprintf("break location %s", lb.locID)
}

type objBreakpoint struct {
	objectID uuid.UUID
}

func (ob objBreakpoint) shouldBreak(e core.Event) bool {
	switch e.Type() {
	case core.EventTypeObjectAddToZone:
		typed := e.(*core.ObjectAddToZoneEvent)
		return uuid.Equal(typed.ObjectID, ob.objectID)
	case core.EventTypeObjectRemoveFromZone:
		typed := e.(*core.ObjectRemoveFromZoneEvent)
		return uuid.Equal(typed.ObjectID, ob.objectID)
	case core.EventTypeObjectMove:
		typed := e.(*core.ObjectMoveEvent)
		if uuid.Equal(typed.ObjectID, ob.objectID) {
			return true
		}
		if uuid.Equal(typed.FromObjectContainerID, ob.objectID) {
			return true
		}
		if uuid.Equal(typed.ToObjectContainerID, ob.objectID) {
			return true
		}
		return false
	case core.EventTypeObjectAdminRelocate:
		typed := e.(*core.ObjectAdminRelocateEvent)
		if uuid.Equal(typed.ObjectID, ob.objectID) {
			return true
		}
		if uuid.Equal(typed.ToObjectContainerID, ob.objectID) {
			return true
		}
		return false
	case core.EventTypeObjectMigrateIn:
		typed := e.(*core.ObjectMigrateInEvent)
		if uuid.Equal(typed.ObjectID, ob.objectID) {
			return true
		}
		if uuid.Equal(typed.ObjectContainerID, ob.objectID) {
			return true
		}
		return false
	case core.EventTypeObjectMigrateOut:
		typed := e.(*core.ObjectMigrateOutEvent)
		return uuid.Equal(typed.ObjectID, ob.objectID)
	default:
		return false
	}
}

func (ob objBreakpoint) String() string {
	return fmt.Sprintf("break object %s", ob.objectID)
}

type timeBreakpoint struct {
	timestamp time.Time
}

func (tb timeBreakpoint) shouldBreak(e core.Event) bool {
	return e.Timestamp().After(tb.timestamp)
}

func (tb timeBreakpoint) String() string {
	return fmt.Sprintf("break time %q", tb.timestamp.Format(timestampLayout))
}

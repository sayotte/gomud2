package main

import (
	"flag"
	"fmt"
	"github.com/sayotte/gomud2/wsapi"
	"log"
	"path/filepath"
	"sync"

	gouuid "github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/core"
	"github.com/sayotte/gomud2/store"
	"github.com/sayotte/gomud2/telnet"
)

var version string

type cliArgs struct {
	initStartingZone bool
	worldConfigFile  string
}

func parseCliArgs() (cliArgs, error) {
	initStartingZone := flag.Bool("initWorld", false, "Create a default world with some locations, edges, objects and actors; persist the related events, then exit.")
	worldConfig := flag.String("config", "mudConfig.yaml", "Configuration file for MUD daemon")

	flag.Parse()

	var args cliArgs

	args.initStartingZone = *initStartingZone
	args.worldConfigFile = *worldConfig

	return args, nil
}

func main() {
	if version == "" {
		version = "<no tag>"
	}
	fmt.Printf("MUD version %s starting\n", version)

	args, err := parseCliArgs()
	if err != nil {
		log.Fatal(err)
	}

	if args.initStartingZone {
		err = initStartingWorld(args.worldConfigFile)
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	cfg := mudConfig{}
	err = (&cfg).DeserializeFromFile(args.worldConfigFile)
	if err != nil {
		log.Fatal(err)
	}

	world := core.NewWorld()
	world.DataStore = &store.EventStore{
		Filename:          cfg.Store.EventsFile,
		UseCompression:    cfg.Store.UseCompression,
		SnapshotDirectory: filepath.Clean(cfg.Store.SnapshotDirectory),
	}
	world.IntentLog = &store.IntentLogger{
		Filename: cfg.Store.IntentLogfile,
	}
	err = world.LoadAndStart(cfg.World.ZonesToLoad, cfg.World.DefaultZoneID, cfg.World.DefaultLocationID)
	if err != nil {
		log.Fatal(err)
	}

	err = runWorld(world, cfg)
	if err != nil {
		log.Fatal(err)
	}

	waitForeverWG := &sync.WaitGroup{}
	waitForeverWG.Add(1)
	waitForeverWG.Wait()
}

func initStartingWorld(worldConfigFile string) error {
	eStore := &store.EventStore{
		Filename:       "store/events.dat",
		UseCompression: true,
	}
	z := core.NewZone(gouuid.Nil, eStore)
	z.StartEventProcessing()

	shortDesc := "A nearby bar"
	longDesc := "Nothing special to see at this local watering hole. Just the usual "
	longDesc += "array of televisions on the wall playing mainstream sports events, one patron "
	longDesc += "at the bar looking desperate to talk to somebody, another at the far end of the bar "
	longDesc += "who appears to be an off-duty cook trying to avoid conversation with anyone, and "
	longDesc += "a female bartender who was probably crazy-hot 15 years ago but is now just crazy."
	loc1Prim := core.NewLocation(gouuid.Nil, z, shortDesc, longDesc)
	loc1, err := z.AddLocation(loc1Prim)
	if err != nil {
		panic(err)
	}

	shortDesc = "123 Elm Street"
	longDesc = "Sitting below the level of the street at the end of a slight "
	longDesc += "slope, this house's cute blue shutters and the whimsical "
	longDesc += "flamingoes in the yard give off a cheerful, playful sense "
	longDesc += "of welcoming."
	loc2Prim := core.NewLocation(gouuid.Nil, z, shortDesc, longDesc)
	loc2, err := z.AddLocation(loc2Prim)
	if err != nil {
		panic(err)
	}

	edge1Prim := core.NewLocationEdge(
		gouuid.Nil,
		"Elm Street",
		core.EdgeDirectionWest,
		loc1,
		loc2,
		z,
		gouuid.Nil,
		gouuid.Nil,
	)
	_, err = z.AddLocationEdge(edge1Prim)
	if err != nil {
		panic(err)
	}

	edge2Prim := core.NewLocationEdge(
		gouuid.Nil,
		"Elm Street",
		core.EdgeDirectionEast,
		loc2,
		loc1,
		z,
		gouuid.Nil,
		gouuid.Nil,
	)
	_, err = z.AddLocationEdge(edge2Prim)
	if err != nil {
		panic(err)
	}

	actorPrim := core.NewActor(gouuid.Nil, "A man", loc1, z)
	_, err = z.AddActor(actorPrim)
	if err != nil {
		panic(err)
	}

	objPrim := core.NewObject(
		gouuid.Nil,
		"a crumpled up napkin",
		loc1,
		z,
	)
	_, err = z.AddObject(objPrim, loc1)
	if err != nil {
		panic(err)
	}

	z2 := core.NewZone(gouuid.Nil, eStore)
	z2.StartEventProcessing()

	shortDesc = "The Foxhunt Room"
	longDesc = "A small room with wood paneled walls, standing here you "
	longDesc += "feel as though you be sitting, sipping tea and making "
	longDesc += "conversation with friends."
	loc3Prim := core.NewLocation(gouuid.Nil, z2, shortDesc, longDesc)
	loc3, err := z2.AddLocation(loc3Prim)
	if err != nil {
		panic(err)
	}

	edge3Prim := core.NewLocationEdge(
		gouuid.Nil,
		"in through the front door",
		core.EdgeDirectionNorth,
		loc2,
		nil,
		z,
		z2.ID(),
		loc3.ID(),
	)
	_, err = z.AddLocationEdge(edge3Prim)
	if err != nil {
		panic(err)
	}

	edge4Prim := core.NewLocationEdge(
		gouuid.Nil,
		"out the front door",
		core.EdgeDirectionSouth,
		loc3,
		nil,
		z2,
		z.ID(),
		loc2.ID(),
	)
	_, err = z2.AddLocationEdge(edge4Prim)
	if err != nil {
		panic(err)
	}

	cfg := mudConfig{
		World: worldConfig{
			DefaultZoneID:     z.ID(),
			DefaultLocationID: loc1.ID(),
			ZonesToLoad: []gouuid.UUID{
				z.ID(),
				z2.ID(),
			},
		},
		Store: storeConfig{
			SnapshotDirectory: "store/snapshots",
			IntentLogfile:     "store/intentlog.dat",
			EventsFile:        "store/events.dat",
			UseCompression:    true,
		},
		Telnet: telnetConfig{
			ListenPort: telnet.DefaultListenPort,
		},
		WSAPI: wsAPIConfig{
			ListenAddr: wsapi.DefaultListenAddr,
		},
	}
	return cfg.SerializeToFile(worldConfigFile)
}

func runWorld(world *core.World, cfg mudConfig) error {
	authServer := &auth.Server{
		AccountDatabaseFile: "auth.db",
	}
	err := authServer.Start()
	if err != nil {
		return err
	}

	telnetServer := telnet.Server{
		ListenPort:      cfg.Telnet.ListenPort,
		MessageQueueLen: telnet.DefaultMessageQueueLen,
		AuthService:     authServer,
		World:           world,
	}
	err = telnetServer.Start()
	if err != nil {
		return err
	}

	apiServer := wsapi.Server{
		ListenAddrString:    cfg.WSAPI.ListenAddr,
		AuthService:         authServer,
		MessageSendQueueLen: wsapi.DefaultMessageSendQueueLen,
		World:               world,
	}
	err = apiServer.Start()
	if err != nil {
		return err
	}

	return nil
}

package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"sync"

	gouuid "github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/core"
	"github.com/sayotte/gomud2/spawnreap"
	"github.com/sayotte/gomud2/store"
	"github.com/sayotte/gomud2/telnet"
	"github.com/sayotte/gomud2/wsapi"
)

var version string

type cliArgs struct {
	initStartingZone bool
	worldConfigFile  string
}

func parseCliArgs() (cliArgs, error) {
	initStartingZone := flag.Bool("initWorld", false, "Create a default world with some locations, exits, objects and actors; persist the related events, then exit.")
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
	z := core.NewZone(gouuid.Nil, "overworld", eStore)
	z.StartCommandProcessing()

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

	err = z.SetDefaultLocation(loc1)
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

	exit1Prim := core.NewExit(
		gouuid.Nil,
		"Elm Street",
		core.ExitDirectionWest,
		loc1,
		loc2,
		z,
		gouuid.Nil,
		gouuid.Nil,
	)
	_, err = z.AddExit(exit1Prim)
	if err != nil {
		panic(err)
	}

	exit2Prim := core.NewExit(
		gouuid.Nil,
		"Elm Street",
		core.ExitDirectionEast,
		loc2,
		loc1,
		z,
		gouuid.Nil,
		gouuid.Nil,
	)
	_, err = z.AddExit(exit2Prim)
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

	z2 := core.NewZone(gouuid.Nil, "123 Elm St", eStore)
	z2.StartCommandProcessing()

	shortDesc = "The Foxhunt Room"
	longDesc = "A small room with wood paneled walls, standing here you "
	longDesc += "feel as though you be sitting, sipping tea and making "
	longDesc += "conversation with friends."
	loc3Prim := core.NewLocation(gouuid.Nil, z2, shortDesc, longDesc)
	loc3, err := z2.AddLocation(loc3Prim)
	if err != nil {
		panic(err)
	}

	err = z2.SetDefaultLocation(loc3)
	if err != nil {
		panic(err)
	}

	exit3Prim := core.NewExit(
		gouuid.Nil,
		"in through the front door",
		core.ExitDirectionNorth,
		loc2,
		nil,
		z,
		z2.ID(),
		loc3.ID(),
	)
	_, err = z.AddExit(exit3Prim)
	if err != nil {
		panic(err)
	}

	exit4Prim := core.NewExit(
		gouuid.Nil,
		"out the front door",
		core.ExitDirectionSouth,
		loc3,
		nil,
		z2,
		z.ID(),
		loc2.ID(),
	)
	_, err = z2.AddExit(exit4Prim)
	if err != nil {
		panic(err)
	}

	spawnSpec := spawnreap.SpawnSpecification{
		ActorProto: spawnreap.ActorPrototype{
			Name: "a rabbit",
		},
		MaxCount:           30,
		MaxSpawnAtOneTime:  10,
		SpawnChancePerTick: 0.5,
	}
	spawnReapSvc := spawnreap.Service{
		World:       &core.World{},
		TickLengthS: int(math.MaxInt64),
		ConfigFile:  spawnreap.DefaultConfigFile,
	}
	err = spawnReapSvc.Start()
	if err != nil {
		panic(err)
	}
	err = spawnReapSvc.PutSpawnConfigForZone(
		[]spawnreap.SpawnSpecification{spawnSpec},
		z,
	)
	if err != nil {
		panic(err)
	}
	spawnReapSvc.Stop()

	cfg := mudConfig{
		World: worldConfig{
			DefaultZoneID:     z.ID(),
			DefaultLocationID: loc1.ID(),
			ZonesToLoad: []string{
				z.Tag(),
				z2.Tag(),
			},
		},
		Store: storeConfig{
			SnapshotDirectory: "store/snapshots",
			IntentLogfile:     "store/intentlog.dat",
			EventsFile:        "store/events.dat",
			UseCompression:    true,
		},
		Telnet: telnetConfig{
			ListenAddr: telnet.DefaultListenAddr,
		},
		WSAPI: wsAPIConfig{
			ListenAddr: wsapi.DefaultListenAddr,
		},
		SpawnReap: spawnReapConfig{
			SpawnsConfigFile:    spawnreap.DefaultConfigFile,
			TicksUntilReap:      spawnreap.DefaultReapTicks,
			TickLengthInSeconds: spawnreap.DefaultTickLengthS,
		},
	}
	return cfg.SerializeToFile(worldConfigFile)
}

func runWorld(world *core.World, cfg mudConfig) error {
	spawnReapService := &spawnreap.Service{
		World:       world,
		ReapTicks:   cfg.SpawnReap.TicksUntilReap,
		TickLengthS: cfg.SpawnReap.TickLengthInSeconds,
	}
	err := spawnReapService.Start()
	if err != nil {
		return err
	}

	authServer := &auth.Server{
		AccountDatabaseFile: "auth.db",
	}
	err = authServer.Start()
	if err != nil {
		return err
	}

	telnetServer := telnet.Server{
		ListenAddr:      cfg.Telnet.ListenAddr,
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

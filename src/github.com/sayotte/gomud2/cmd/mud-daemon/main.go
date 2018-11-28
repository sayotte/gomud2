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
	"github.com/sayotte/gomud2/domain"
	"github.com/sayotte/gomud2/store"
	"github.com/sayotte/gomud2/telnet"
	"github.com/sayotte/gomud2/uuid"
)

const (
	TelnetServerEventQueueLen      = 15
	WebsocketServerMessageQueueLen = 15
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

	eStore := &store.EventStore{
		Filename:          cfg.Store.EventsFile,
		UseCompression:    cfg.Store.UseCompression,
		SnapshotDirectory: filepath.Clean(cfg.Store.SnapshotDirectory),
	}

	world, err := loadWorld(cfg, eStore)
	if err != nil {
		log.Fatal(err)
	}

	err = runWorld(world)
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
	z := domain.NewZone(eStore)
	z.StartEventProcessing()

	shortDesc := "A nearby bar"
	longDesc := "Nothing special to see at this local watering hole. Just the usual "
	longDesc += "array of televisions on the wall playing mainstream sports events, one patron "
	longDesc += "at the bar looking desperate to talk to somebody, another at the far end of the bar "
	longDesc += "who appears to be an off-duty cook trying to avoid conversation with anyone, and "
	longDesc += "a female bartender who was probably crazy-hot 15 years ago but is now just crazy."
	loc1Prim := domain.NewLocation(z, shortDesc, longDesc)
	loc1, err := z.AddLocation(loc1Prim)
	if err != nil {
		panic(err)
	}

	shortDesc = "123 Elm Street"
	longDesc = "Sitting below the level of the street at the end of a slight "
	longDesc += "slope, this house's cute blue shutters and the whimsical "
	longDesc += "flamingoes in the yard give off a cheerful, playful sense "
	longDesc += "of welcoming."
	loc2Prim := domain.NewLocation(z, shortDesc, longDesc)
	loc2, err := z.AddLocation(loc2Prim)
	if err != nil {
		panic(err)
	}

	edge1Prim := domain.NewLocationEdge(
		uuid.NewId(),
		"Elm Street",
		domain.EdgeDirectionWest,
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

	edge2Prim := domain.NewLocationEdge(
		uuid.NewId(),
		"Elm Street",
		domain.EdgeDirectionEast,
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

	actorPrim := domain.NewActor("A man", loc1, z)
	_, err = z.AddActor(actorPrim)
	if err != nil {
		panic(err)
	}

	objPrim := domain.NewObject(
		uuid.NewId(),
		"a crumpled up napkin",
		loc1,
		z,
	)
	_, err = z.AddObject(objPrim, loc1)
	if err != nil {
		panic(err)
	}

	z2 := domain.NewZone(eStore)
	z2.StartEventProcessing()

	shortDesc = "The Foxhunt Room"
	longDesc = "A small room with wood paneled walls, standing here you "
	longDesc += "feel as though you be sitting, sipping tea and making "
	longDesc += "conversation with friends."
	loc3Prim := domain.NewLocation(z2, shortDesc, longDesc)
	loc3, err := z2.AddLocation(loc3Prim)
	if err != nil {
		panic(err)
	}

	edge3Prim := domain.NewLocationEdge(
		uuid.NewId(),
		"in through the front door",
		domain.EdgeDirectionNorth,
		loc2,
		nil,
		z,
		z2.Id,
		loc3.Id,
	)
	_, err = z.AddLocationEdge(edge3Prim)
	if err != nil {
		panic(err)
	}

	edge4Prim := domain.NewLocationEdge(
		uuid.NewId(),
		"out the front door",
		domain.EdgeDirectionSouth,
		loc3,
		nil,
		z2,
		z.Id,
		loc2.Id,
	)
	_, err = z2.AddLocationEdge(edge4Prim)
	if err != nil {
		panic(err)
	}

	cfg := mudConfig{
		World: worldConfig{
			DefaultZoneID:     z.Id,
			DefaultLocationID: loc1.Id,
			ZonesToLoad: []gouuid.UUID{
				z.Id,
				z2.Id,
			},
		},
		Store: storeConfig{
			SnapshotDirectory: "store/snapshots",
			IntentLogfile:     "store/intentlog.dat",
			EventsFile:        "store/events.dat",
			UseCompression:    true,
		},
	}
	return cfg.SerializeToFile(worldConfigFile)
}

func loadWorld(cfg mudConfig, eStore domain.DoEverythingStore) (*domain.World, error) {
	world := domain.NewWorld()
	world.SnapshotHelper = eStore
	world.IntentLog = &store.IntentLogger{
		Filename: cfg.Store.IntentLogfile,
	}
	err := world.Start()
	if err != nil {
		return nil, err
	}

	for _, zoneID := range cfg.World.ZonesToLoad {
		z := domain.NewZone(nil)
		z.Id = zoneID
		err := world.AddZone(z)
		if err != nil {
			return nil, err
		}

		eChan, err := eStore.RetrieveAllForZone(zoneID)
		if err != nil {
			return nil, err
		}
		err = z.ReplayEvents(eChan)
		if err != nil {
			return nil, err
		}

		z.SetPersister(eStore)
	}

	defaultZone := world.ZoneByID(cfg.World.DefaultZoneID)
	if defaultZone == nil {
		return nil, fmt.Errorf("default Zone %q not found in World", cfg.World.DefaultZoneID)
	}
	defaultLoc := defaultZone.LocationByID(cfg.World.DefaultLocationID)
	if defaultLoc == nil {
		return nil, fmt.Errorf("default Location %q not found in default Zone", cfg.World.DefaultLocationID)
	}
	world.FrontDoorZone = defaultZone
	world.FrontDoorLocation = defaultLoc

	for _, zone := range world.Zones() {
		zone.StartEventProcessing()
	}

	err = world.ReplayIntentLog()
	if err != nil {
		return nil, err
	}

	return world, nil
}

func runWorld(world *domain.World) error {
	authServer := &auth.Server{
		AccountDatabaseFile: "auth.db",
	}
	err := authServer.Start()
	if err != nil {
		return err
	}

	telnetServer := telnet.Server{
		ListenPort:      4000,
		MessageQueueLen: TelnetServerEventQueueLen,
		AuthService:     authServer,
		World:           world,
	}
	err = telnetServer.Start()
	if err != nil {
		return err
	}

	apiServer := wsapi.Server{
		AuthService:      authServer,
		ListenAddrString: ":4001",
		World:            world,
	}
	err = apiServer.Start()
	if err != nil {
		return err
	}

	return nil
}

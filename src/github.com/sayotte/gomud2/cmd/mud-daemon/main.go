package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"path/filepath"
	"runtime/pprof"
	"sync"

	gouuid "github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/brain"
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
	cpuProfile       string
}

func parseCliArgs() (cliArgs, error) {
	initStartingZone := flag.Bool("initWorld", false, "Create a default world with some locations, exits, objects and actors; persist the related events, then exit.")
	worldConfig := flag.String("config", "mudConfig.yaml", "Configuration file for MUD daemon")
	cpuProfile := flag.String("cpuprofile", "", "Write CPU profile information to this file")

	flag.Parse()

	var args cliArgs

	args.initStartingZone = *initStartingZone
	args.worldConfigFile = *worldConfig
	args.cpuProfile = *cpuProfile

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

	if args.cpuProfile != "" {
		f, err := os.Create(args.cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
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
	//time.Sleep(1 * time.Minute)
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
		"This was once the sort of napkin that bartenders put down so your drink doesn't leave a wet ring on the bar. Now it's crumpled into a ball.",
		[]string{"napkin"},
		loc1,
		0,
		z,
	)
	_, err = z.AddObject(objPrim, loc1)
	if err != nil {
		panic(err)
	}

	bagPrim := core.NewObject(
		gouuid.Nil,
		"a shopping bag",
		"A brown-paper bag, with the little twisted-paper handles that bougie department stores like to use so their customers can feel like they're not harming the environment when they purchase products made of processed, bleached baby animal souls.",
		[]string{"bag"},
		loc1,
		20,
		z,
	)
	_, err = z.AddObject(bagPrim, loc1)
	if err != nil {
		panic(err)
	}

	z2 := core.NewZone(gouuid.Nil, "123 Elm St", eStore)
	z2.StartCommandProcessing()

	shortDesc = "The Foxhunt Room"
	longDesc = "A small room with wood paneled walls, standing here you "
	longDesc += "feel as though you should be sitting, sipping tea and "
	longDesc += "making conversation with friends."
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

	chessboardZone, a1Loc, err := makeChessboard(eStore)
	if err != nil {
		panic(err)
	}
	exitToChessboardPrim := core.NewExit(
		gouuid.Nil,
		"into a game of wizard's chess...",
		core.ExitDirectionNorth,
		loc1,
		nil,
		z,
		chessboardZone.ID(),
		a1Loc.ID(),
	)
	_, err = z.AddExit(exitToChessboardPrim)
	if err != nil {
		panic(err)
	}
	exitFromChessboardPrim := core.NewExit(
		gouuid.Nil,
		"out of the wizard's chessboard",
		core.ExitDirectionSouth,
		a1Loc,
		nil,
		chessboardZone,
		z.ID(),
		loc1.ID(),
	)
	_, err = chessboardZone.AddExit(exitFromChessboardPrim)
	if err != nil {
		panic(err)
	}

	spawnSpec := spawnreap.SpawnSpecification{
		ActorProto: spawnreap.ActorPrototype{
			Name:      "a rabbit",
			BrainType: "crowd-averse-wanderer",
		},
		MaxCount:           30,
		MaxSpawnAtOneTime:  10,
		SpawnChancePerTick: 0.5,
	}
	chessSpawnSpec := spawnreap.SpawnSpecification{
		ActorProto: spawnreap.ActorPrototype{
			Name:      "a chess piece",
			BrainType: "crowd-averse-wanderer",
		},
		MaxCount:           16,
		MaxSpawnAtOneTime:  16,
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
	err = spawnReapSvc.PutSpawnConfigForZone(
		[]spawnreap.SpawnSpecification{chessSpawnSpec},
		chessboardZone,
	)
	if err != nil {
		panic(err)
	}
	spawnReapSvc.Stop()

	cfg := mudConfig{
		World: worldConfig{
			DefaultZoneID:     z.ID(),
			DefaultLocationID: loc1.ID(),
			//DefaultZoneID:     chessboardZone.ID(),
			//DefaultLocationID: a1Loc.ID(),
			ZonesToLoad: []string{
				z.Tag(),
				z2.Tag(),
				chessboardZone.Tag(),
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

func makeChessboard(eStore *store.EventStore) (*core.Zone, *core.Location, error) {
	z := core.NewZone(gouuid.Nil, "wizard's chessboard", eStore)
	z.StartCommandProcessing()

	switchSquareColor := func(currentColor string) string {
		if currentColor == "black" {
			return "white"
		}
		return "black"
	}

	squareNamesToLocations := make(map[string]*core.Location, 64)

	colNames := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
	maxRows := 8
	squareColor := "black"
	for rowNum := 1; rowNum <= maxRows; rowNum++ {
		for colNameIdx := 0; colNameIdx < len(colNames); colNameIdx++ {
			colName := colNames[colNameIdx]
			squareName := fmt.Sprintf("%s%d", colName, rowNum)
			//fmt.Printf("square: %s (%s)\n", squareName, squareColor)

			shortDesc := fmt.Sprintf("Square %s", squareName)
			longDesc := fmt.Sprintf("This is a %s square.", squareColor)
			locPrim := core.NewLocation(gouuid.Nil, z, shortDesc, longDesc)
			loc, err := z.AddLocation(locPrim)
			if err != nil {
				return nil, nil, err
			}
			squareNamesToLocations[squareName] = loc

			// if we're not at the left edge, link west
			if colNameIdx > 0 {
				linkName := fmt.Sprintf("%s%d", colNames[colNameIdx-1], rowNum)
				//fmt.Printf("\tleft -> %s\n", linkName)
				linkLoc := squareNamesToLocations[linkName]
				err = doBasicBidirectionalExits(loc, linkLoc, core.ExitDirectionWest, z)
				if err != nil {
					return nil, nil, err
				}
			}
			// if we're not at the bottom edge, link south
			if rowNum > 1 {
				linkName := fmt.Sprintf("%s%d", colName, rowNum-1)
				//fmt.Printf("\tdown -> %s\n", linkName)
				linkLoc := squareNamesToLocations[linkName]
				err = doBasicBidirectionalExits(loc, linkLoc, core.ExitDirectionSouth, z)
				if err != nil {
					return nil, nil, err
				}
			}

			squareColor = switchSquareColor(squareColor)
		}
		squareColor = switchSquareColor(squareColor)
	}

	a1Loc := squareNamesToLocations["A1"]
	err := z.SetDefaultLocation(a1Loc)
	if err != nil {
		return nil, nil, err
	}

	return z, a1Loc, nil
}

func doBasicBidirectionalExits(fromLoc, toLoc *core.Location, dir string, z *core.Zone) error {
	exitPrim := core.NewExit(
		gouuid.Nil,
		fmt.Sprintf("To square %s", toLoc.ShortDescription()),
		dir,
		fromLoc,
		toLoc,
		z,
		gouuid.Nil,
		gouuid.Nil,
	)
	_, err := z.AddExit(exitPrim)
	if err != nil {
		return err
	}

	var returnDir string
	switch dir {
	case core.ExitDirectionEast:
		returnDir = core.ExitDirectionWest
	case core.ExitDirectionWest:
		returnDir = core.ExitDirectionEast
	case core.ExitDirectionNorth:
		returnDir = core.ExitDirectionSouth
	case core.ExitDirectionSouth:
		returnDir = core.ExitDirectionNorth
	}
	exitPrim = core.NewExit(
		gouuid.Nil,
		fmt.Sprintf("To square %s", fromLoc.ShortDescription()),
		returnDir,
		toLoc,
		fromLoc,
		z,
		gouuid.Nil,
		gouuid.Nil,
	)
	_, err = z.AddExit(exitPrim)
	if err != nil {
		return err
	}

	return nil
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

	brainService := &brain.Service{
		AuthUsername:   "a",
		AuthPassword:   "a",
		WSAPIURLString: "ws://localhost:4001",
	}

	spawnReapService := &spawnreap.Service{
		World:       world,
		BrainSvc:    brainService,
		ReapTicks:   cfg.SpawnReap.TicksUntilReap,
		TickLengthS: cfg.SpawnReap.TickLengthInSeconds,
	}
	err = spawnReapService.Start()
	if err != nil {
		return err
	}

	return nil
}

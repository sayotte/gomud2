package commands

import (
	"errors"
	"github.com/sayotte/gomud2/core"
)

func MoveActor(actor *core.Actor, direction string, observer core.Observer) (*core.Actor, error) {
	var outExit *core.Exit
	for _, exit := range actor.Location().OutExits() {
		if exit.Direction() == direction {
			outExit = exit
			break
		}
	}
	if outExit == nil {
		return actor, errors.New(ErrorNoSuchExit)
	}

	// Intra-zone move
	if outExit.Destination() != nil {
		err := actor.Move(actor.Location(), outExit.Destination())
		if err != nil {
			return nil, err
		}
		return actor, nil
	}

	// Inter-zone migration
	world := actor.Zone().World()
	remoteZone := world.ZoneByID(outExit.OtherZoneID())
	if remoteZone == nil {
		return nil, errors.New(ErrorMigrationFailed)
	}
	remoteLoc := remoteZone.LocationByID(outExit.OtherZoneLocID())
	if remoteLoc == nil {
		return nil, errors.New(ErrorMigrationFailed)
	}
	newActor, err := world.MigrateActor(actor, actor.Location(), remoteLoc)
	if err != nil {
		return actor, errors.New(ErrorMigrationFailed)
	}
	newActor.AddObserver(observer)
	return newActor, nil
}

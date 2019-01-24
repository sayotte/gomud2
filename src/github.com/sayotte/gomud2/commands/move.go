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
	newActor, err := actor.Zone().World().MigrateActor(actor, actor.Zone(), outExit.OtherZoneID(), outExit.OtherZoneLocID(), observer)
	if err != nil {
		return actor, errors.New(ErrorMigrationFailed)
	}
	newActor.AddObserver(observer)
	return newActor, nil
}

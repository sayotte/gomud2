package commands

import (
	"errors"
	"github.com/sayotte/gomud2/domain"
)

func MoveActor(actor *domain.Actor, direction string, observer domain.Observer) (*domain.Actor, error) {
	var outEdge *domain.LocationEdge
	for _, edge := range actor.Location().OutEdges {
		if edge.Direction == direction {
			outEdge = edge
			break
		}
	}
	if outEdge == nil {
		return actor, errors.New(ErrorNoSuchExit)
	}

	// Intra-zone move
	if outEdge.Destination != nil {
		err := actor.Move(actor.Location(), outEdge.Destination)
		if err != nil {
			return nil, err
		}
		return actor, nil
	}

	// Inter-zone migration
	newActor, err := actor.Zone().World().MigrateActor(actor, actor.Zone(), outEdge.OtherZoneID, outEdge.OtherZoneLocID, observer)
	if err != nil {
		return actor, errors.New(ErrorMigrationFailed)
	}
	newActor.AddObserver(observer)
	return newActor, nil
}

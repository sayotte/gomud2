package commands

import (
	"fmt"
	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/core"
)

func LookAtLocationByID(locID, zoneID uuid.UUID, world *core.World) (LocationInfo, error) {
	zone := world.ZoneByID(zoneID)
	if zone == nil {
		return LocationInfo{}, fmt.Errorf("no such Zone %q", zoneID)
	}
	loc := zone.LocationByID(locID)
	if loc == nil {
		return LocationInfo{}, fmt.Errorf("no such Location %q", locID)
	}
	return LookAtLocation(loc), nil
}

func LookAtLocation(loc *core.Location) LocationInfo {
	lInfo := LocationInfo{
		ID:               loc.ID(),
		ZoneID:           loc.Zone().ID(),
		ShortDescription: loc.ShortDescription(),
		Description:      loc.Description(),
	}
	for _, a := range loc.Actors() {
		lInfo.Actors = append(lInfo.Actors, a.ID())
	}
	for _, o := range loc.Objects() {
		lInfo.Objects = append(lInfo.Objects, o.ID())
	}
	lInfo.Exits = make(map[string][2]uuid.UUID)
	for _, ex := range loc.OutExits() {
		if ex.Destination() != nil {
			lInfo.Exits[ex.Direction()] = [2]uuid.UUID{
				loc.Zone().ID(),
				ex.Destination().ID(),
			}
		} else {
			lInfo.Exits[ex.Direction()] = [2]uuid.UUID{
				ex.OtherZoneID(),
				ex.OtherZoneLocID(),
			}
		}
	}
	return lInfo
}

type LocationInfo struct {
	ID               uuid.UUID
	ZoneID           uuid.UUID
	ShortDescription string
	Description      string
	Actors           []uuid.UUID
	Objects          []uuid.UUID
	Exits            map[string][2]uuid.UUID // direction->ZoneID/LocationID
}

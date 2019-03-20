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

func LookAtActor(actor *core.Actor) ActorVisibleInfo {
	aInfo := ActorVisibleInfo{
		ID:               actor.ID(),
		Name:             actor.Name(),
		VisibleInventory: make(map[string][]uuid.UUID, len(core.AllActorInventorySubcontainers)),
	}
	for _, subContainerName := range core.AllActorInventorySubcontainers {
		var contents []uuid.UUID
		for _, obj := range actor.Inventory().ObjectsBySubcontainer(subContainerName) {
			contents = append(contents, obj.ID())
		}
		aInfo.VisibleInventory[subContainerName] = contents
	}
	attrs := actor.Attributes()
	aInfo.VisibleAttributes = ActorVisibleAttributes{
		Strength:       attrs.Strength,
		Physical:       attrs.Physical,
		Fitness:        attrs.Fitness,
		Stamina:        attrs.Stamina,
		Will:           attrs.Will,
		Focus:          attrs.Focus,
		Faith:          attrs.Faith,
		Zeal:           attrs.Zeal,
		NaturalBiteMin: attrs.NaturalBiteMin,
		NaturalBiteMax: attrs.NaturalBiteMax,
	}
	return aInfo
}

type ActorVisibleInfo struct {
	ID                uuid.UUID
	Name              string
	VisibleInventory  map[string][]uuid.UUID
	VisibleAttributes ActorVisibleAttributes
}

type ActorVisibleAttributes struct {
	Strength, Physical               int
	Fitness, Stamina                 int
	Will, Focus                      int
	Faith, Zeal                      int
	NaturalBiteMin, NaturalBiteMax   float64
	NaturalSlashMin, NaturalSlashMax float64
}

func LookAtObject(obj *core.Object) ObjectVisibleInfo {
	info := ObjectVisibleInfo{
		ID:          obj.ID(),
		Name:        obj.Name(),
		Description: obj.Description(),
		Capacity:    obj.InventorySlots(), // FIXME or should it be obj.containerCapacity?
		Attributes:  obj.Attributes(),
	}
	for _, subObj := range obj.Objects() {
		info.ContainedObjects = append(info.ContainedObjects, subObj.ID())
	}
	return info
}

type ObjectVisibleInfo struct {
	ID               uuid.UUID
	Name             string
	Description      string
	Capacity         int
	ContainedObjects []uuid.UUID
	Attributes       core.ObjectAttributes
}

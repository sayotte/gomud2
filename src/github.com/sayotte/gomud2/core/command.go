package core

type Command interface {
	CommandType() int
}

const (
	CommandTypeActorMove = iota
	CommandTypeActorAdminRelocate
	CommandTypeActorAddToZone
	CommandTypeActorRemoveFromZone
	CommandTypeLocationAddToZone
	CommandTypeLocationRemoveFromZone
	CommandTypeLocationUpdate
	CommandTypeExitAddToZone
	CommandTypeExitUpdate
	CommandTypeExitRemoveFromZone
	CommandTypeObjectAddToZone
	CommandTypeObjectRemoveFromZone
	CommandTypeObjectMove
	CommandTypeObjectAdminRelocate
	CommandTypeZoneSetDefaultLocation
)

type commandGeneric struct {
	commandType int
}

func (cg commandGeneric) CommandType() int {
	return cg.commandType
}

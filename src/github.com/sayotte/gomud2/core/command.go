package core

type Command interface {
	CommandType() int
}

const (
	CommandTypeActorAddToZone = iota
	CommandTypeActorMove
	CommandTypeActorAdminRelocate
	CommandTypeActorRemoveFromZone
	CommandTypeLocationAddToZone
	CommandTypeLocationUpdate
	CommandTypeLocationRemoveFromZone
	CommandTypeExitAddToZone
	CommandTypeExitUpdate
	CommandTypeExitRemoveFromZone
	CommandTypeObjectAddToZone
	CommandTypeObjectMove
	CommandTypeObjectAdminRelocate
	CommandTypeObjectRemoveFromZone
	CommandTypeZoneSetDefaultLocation
)

type commandGeneric struct {
	commandType int
}

func (cg commandGeneric) CommandType() int {
	return cg.commandType
}

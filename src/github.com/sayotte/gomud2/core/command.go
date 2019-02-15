package core

type Command interface {
	CommandType() int
}

const (
	CommandTypeActorAddToZone = iota
	CommandTypeActorMove
	CommandTypeActorAdminRelocate
	CommandTypeActorRemoveFromZone
	CommandTypeActorMigrateIn
	CommandTypeActorMigrateOut
	CommandTypeLocationAddToZone
	CommandTypeLocationUpdate
	CommandTypeLocationRemoveFromZone
	CommandTypeExitAddToZone
	CommandTypeExitUpdate
	CommandTypeExitRemoveFromZone
	CommandTypeObjectAddToZone
	CommandTypeObjectMove
	CommandTypeObjectMoveSubcontainer
	CommandTypeObjectAdminRelocate
	CommandTypeObjectRemoveFromZone
	CommandTypeZoneSetDefaultLocation
	CommandTypeCombatMelee
)

type commandGeneric struct {
	commandType int
}

func (cg commandGeneric) CommandType() int {
	return cg.commandType
}

package intelligence

import (
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/commands"
	"github.com/sayotte/gomud2/core"
	uuid2 "github.com/sayotte/gomud2/uuid"
	"math"
	"time"

	"github.com/sayotte/gomud2/brain/internal/inner/pathfind"
)

// Necessary because right now we only have the same basic "visible" info on
// ourselves as we do for all other Actors, i.e. we can't see HandMaxItems.
const hokeyStaticMaxHandItems = 2

var staticMeleeDelay = float64(core.ActorMeleeDelay) / float64(time.Second)

type combatPlan struct {
	actorID uuid.UUID

	availableActions []CombatAction

	expansions int
	nextSeqID  int

	plannedActions  []CombatAction
	executionIndex  int
	executionStatus int
}

func (cp *combatPlan) plan(memory *Memory) {
	var startingState combatState
	(&startingState).initFromMemory(cp.actorID, uuid.UUID(memory.GetLastAttacker()), memory)
	startingAction := &startingAction{
		finalState: startingState,
	}

	coster := func(src, dst interface{}) float64 {
		action := dst.(CombatAction)
		return action.TimeToExecute()
	}

	//estimator := func(n interface{}) float64 {
	//	action := n.(CombatAction)
	//	return action.Estimate(memory)
	//}

	isGoaler := func(n interface{}) bool {
		action := n.(CombatAction)
		return action.FinalState().EnemyPhys <= 0
	}

	nGen := func(n interface{}) []interface{} {
		startingState := n.(CombatAction).FinalState()
		actions := cp.genActionsForState(startingState, memory)
		ret := make([]interface{}, 0, len(actions))
		for _, action := range actions {
			ret = append(ret, action)
		}
		return ret
	}

	cp.plannedActions = []CombatAction{}

	pathFindStart := time.Now()
	cameFromMap, _, finalAction := pathfind.DijkstraFindPath(startingAction, coster, isGoaler, nGen, 4000)
	pathFindRuntime := time.Since(pathFindStart)

	// it's possible the path-finding algorithm didn't find any solution within
	// the bounds we gave it; if so, return immediately
	// FIXME use A* rather than Dijkstra's, and modify it to return a partial
	// FIXME path rather than failing entirely
	_, ok := finalAction.(CombatAction)
	if !ok {
		fmt.Println("BRAIN DEBUG: combatPlan failed to find a solution, doing nothing")
		return
	}

	// walk backwards to rebuild our full path
	currentAction := finalAction
	for currentAction != nil {
		cp.plannedActions = append(cp.plannedActions, currentAction.(CombatAction))
		currentAction = cameFromMap[currentAction]
	}

	// discard the last (actually first) node, as this is a *startingAction{}
	// that does nothing
	cp.plannedActions = cp.plannedActions[:len(cp.plannedActions)-1]

	// reverse the path so it's in execution-order
	// see: https://github.com/golang/go/wiki/SliceTricks#reversing
	for i := len(cp.plannedActions)/2 - 1; i >= 0; i-- {
		opp := len(cp.plannedActions) - 1 - i
		cp.plannedActions[i], cp.plannedActions[opp] = cp.plannedActions[opp], cp.plannedActions[i]
	}

	fmt.Println("Action Plan:")
	for i, action := range cp.plannedActions {
		fmt.Printf("\t%.2d: %T\n", i, action)
	}
	//fmt.Printf("Total cost: %.2f\n", costSoFarMap[finalAction])
	fmt.Printf("Expansions: %d\nTotal nodes generated: %d\n", cp.expansions, cp.nextSeqID)
	fmt.Printf("Pathfinding runtime: %s\n", pathFindRuntime)
}

func (cp *combatPlan) genActionsForState(startingState combatState, memory *Memory) []CombatAction {
	var validActions []CombatAction
	for _, action := range cp.availableActions {
		validActions = append(validActions, action.CloneForAllValidTargets(startingState, memory)...)
	}
	cp.expansions += 1
	return validActions
}

func (cp *combatPlan) executeStep(msgSender MessageSender, intellect *Intellect) {
	if cp.executionStatus == executionPlanStatusFailed || cp.executionStatus == executionPlanStatusComplete {
		return
	}
	if cp.executionIndex >= len(cp.plannedActions) || len(cp.plannedActions) == 0 {
		cp.executionStatus = executionPlanStatusComplete
		return
	}

	action := cp.plannedActions[cp.executionIndex]
	cp.executionIndex += 1

	err := action.Execute(msgSender, intellect)
	if err != nil {
		cp.executionStatus = executionPlanStatusFailed
	}
}

func (cp *combatPlan) status() int {
	return cp.executionStatus
}

type combatState struct {
	TargetID            uuid.UUID
	EnemyPhys           float64
	AvgSlashDamage      float64
	AvgBiteDamage       float64
	TimeToHit           float64
	SelfInfo            commands.ActorVisibleInfo
	CurrentLocationInfo commands.LocationInfo
}

func (cs *combatState) initFromMemory(selfID, targetID uuid.UUID, memory *Memory) {
	*cs = combatState{} // init to zero state

	cs.TargetID = targetID

	targetInfo, _ := memory.GetActorInfo(targetID)
	cs.EnemyPhys = float64(targetInfo.VisibleAttributes.Physical)
	cs.TimeToHit = float64(core.ActorMeleeDelay) / float64(time.Second)

	currentZoneID, currentLocID := memory.GetCurrentZoneAndLocationID()
	locInfo, err := memory.GetLocationInfo(currentZoneID, currentLocID)
	if err != nil {
		fmt.Printf("BRAIN ERROR: %s\n", err)
		return
	}
	cs.CurrentLocationInfo = locInfo

	cs.SelfInfo, err = memory.GetActorInfo(selfID)
	if err != nil {
		fmt.Printf("BRAIN ERROR: %s\n", err)
		return
	}

	var bestAvgSlashDmg float64
	for _, objID := range cs.SelfInfo.VisibleInventory[core.InventoryContainerHands] {
		objInfo, err := memory.GetObjectInfo(objID)
		if err != nil {
			fmt.Printf("BRAIN ERROR: %s\n", err)
			return
		}
		avgSlashDmg := (objInfo.Attributes.SlashingDamageMin + objInfo.Attributes.SlashingDamageMax) / 2
		if avgSlashDmg > bestAvgSlashDmg {
			bestAvgSlashDmg = avgSlashDmg
		}
	}
	cs.AvgSlashDamage = bestAvgSlashDmg

	cs.AvgBiteDamage = (cs.SelfInfo.VisibleAttributes.NaturalBiteMin + cs.SelfInfo.VisibleAttributes.NaturalBiteMax) / 2
}

type CombatAction interface {
	CloneForAllValidTargets(startingState combatState, memory *Memory) []CombatAction
	FinalState() combatState
	TimeToExecute() float64
	String() string
	Estimate(memory *Memory) float64
	Execute(msgSender MessageSender, intellect *Intellect) error
}

var allActionsBase = []CombatAction{
	&slashAction{},
	&biteAction{},
	&takeObjectAction{},
}

type startingAction struct {
	finalState combatState
	seqID      int
}

func (sa *startingAction) CloneForAllValidTargets(startingState combatState, memory *Memory) []CombatAction {
	return nil
}

func (sa *startingAction) FinalState() combatState {
	return sa.finalState
}

func (sa *startingAction) TimeToExecute() float64 {
	return 0.0
}

func (sa *startingAction) String() string {
	return fmt.Sprintf("%T[%d]", sa, sa.seqID)
}

func (sa *startingAction) Estimate(memory *Memory) float64 {
	return math.MaxFloat64
}

func (sa *startingAction) Execute(msgSender MessageSender, intellect *Intellect) error {
	return nil
}

type slashAction struct {
	finalState combatState
}

func (sa slashAction) FinalState() combatState {
	return sa.finalState
}

func (sa slashAction) TimeToExecute() float64 {
	return staticMeleeDelay
}

func (sa slashAction) String() string {
	return fmt.Sprintf("%T[%s]", sa, sa.finalState.TargetID)
}

func (sa slashAction) Estimate(memory *Memory) float64 {
	return sa.finalState.EnemyPhys / sa.finalState.AvgSlashDamage * staticMeleeDelay
}

func (sa slashAction) Execute(msgSender MessageSender, intellect *Intellect) error {
	return melee(core.CombatMeleeDamageTypeSlash, sa.finalState.TargetID, msgSender, intellect)
}

func (sa slashAction) CloneForAllValidTargets(startingState combatState, memory *Memory) []CombatAction {
	if startingState.SelfInfo.VisibleAttributes.Physical <= 0 {
		return nil
	}

	newState := startingState
	newState.EnemyPhys -= startingState.AvgSlashDamage
	return []CombatAction{&slashAction{
		finalState: newState,
	}}
}

type biteAction struct {
	finalState combatState
}

func (ba biteAction) CloneForAllValidTargets(startingState combatState, memory *Memory) []CombatAction {
	if startingState.SelfInfo.VisibleAttributes.Physical <= 0 {
		return nil
	}
	if startingState.SelfInfo.VisibleAttributes.NaturalBiteMax <= 0 {
		return nil
	}

	newState := startingState
	newState.EnemyPhys -= startingState.AvgBiteDamage
	return []CombatAction{&biteAction{
		finalState: newState,
	}}
}

func (ba biteAction) FinalState() combatState {
	return ba.finalState
}

func (ba biteAction) TimeToExecute() float64 {
	return staticMeleeDelay
}

func (ba biteAction) String() string {
	return fmt.Sprintf("%T[%s]", ba, ba.finalState.TargetID)
}

func (ba biteAction) Estimate(memory *Memory) float64 {
	return ba.finalState.EnemyPhys / ba.finalState.AvgBiteDamage * staticMeleeDelay
}

func (ba biteAction) Execute(msgSender MessageSender, intellect *Intellect) error {
	return melee(core.CombatMeleeDamageTypeBite, ba.finalState.TargetID, msgSender, intellect)
}

type takeObjectAction struct {
	objectID       uuid.UUID
	fromLocationID uuid.UUID
	fromObjectID   uuid.UUID

	finalState combatState
}

func (toa *takeObjectAction) Execute(msgSender MessageSender, intellect *Intellect) error {
	return moveObject(
		toa.objectID,
		toa.fromLocationID,
		uuid.Nil,
		toa.fromObjectID,
		uuid.Nil,
		toa.finalState.SelfInfo.ID,
		uuid.Nil,
		core.ContainerDefaultSubcontainer,
		msgSender,
		intellect,
	)
}

func (toa *takeObjectAction) CloneForAllValidTargets(startingState combatState, memory *Memory) []CombatAction {
	if startingState.SelfInfo.VisibleAttributes.Physical <= 0 {
		return nil
	}
	if len(startingState.SelfInfo.VisibleInventory[core.InventoryContainerHands]) >= hokeyStaticMaxHandItems {
		return nil
	}

	makeTOA := func(objID, fromLocID, fromObjID uuid.UUID) (*takeObjectAction, error) {
		newState := startingState

		objInfo, err := memory.GetObjectInfo(objID)
		if err != nil {
			return nil, err
		}
		objAvgSlashDamage := (objInfo.Attributes.SlashingDamageMin + objInfo.Attributes.SlashingDamageMax) / 2
		if objAvgSlashDamage > newState.AvgSlashDamage {
			newState.AvgSlashDamage = objAvgSlashDamage
		}

		newState.CurrentLocationInfo.Objects = uuid2.UUIDList(newState.CurrentLocationInfo.Objects).Remove(objID)
		newHandsItems := newState.SelfInfo.VisibleInventory[core.InventoryContainerHands]
		newHandsItems = append(newHandsItems, objID)
		newState.SelfInfo.VisibleInventory[core.InventoryContainerHands] = newHandsItems
		newToa := &takeObjectAction{
			objectID:       objID,
			fromLocationID: fromLocID,
			fromObjectID:   fromObjID,
			finalState:     newState,
		}
		return newToa, nil
	}

	var out []CombatAction
	// First, all objects lying on the ground
	for _, objID := range startingState.CurrentLocationInfo.Objects {
		newToa, err := makeTOA(objID, startingState.CurrentLocationInfo.ID, uuid.Nil)
		if err != nil {
			fmt.Printf("BRAIN ERROR: %s\n", err)
			return nil
		}
		out = append(out, newToa)
	}
	// Second, objects in containers on the ground
	for _, fromObjID := range startingState.CurrentLocationInfo.Objects {
		fromObjInfo, err := memory.GetObjectInfo(fromObjID)
		if err != nil {
			fmt.Printf("BRAIN ERROR: %s\n", err)
			return nil
		}
		for _, objID := range fromObjInfo.ContainedObjects {
			newToa, err := makeTOA(objID, uuid.Nil, fromObjID)
			if err != nil {
				fmt.Printf("BRAIN ERROR: %s\n", err)
				return nil
			}
			out = append(out, newToa)
		}
	}
	// Third, objects in containers on our person
	for _, objList := range startingState.SelfInfo.VisibleInventory {
		for _, fromObjID := range objList {
			fromObjInfo, err := memory.GetObjectInfo(fromObjID)
			if err != nil {
				fmt.Printf("BRAIN ERROR: %s\n", err)
				return nil
			}
			for _, objID := range fromObjInfo.ContainedObjects {
				newToa, err := makeTOA(objID, uuid.Nil, fromObjID)
				if err != nil {
					fmt.Printf("BRAIN ERROR: %s\n", err)
					return nil
				}
				out = append(out, newToa)
			}
		}
	}

	return out
}

func (toa *takeObjectAction) FinalState() combatState {
	return toa.finalState
}

func (toa *takeObjectAction) TimeToExecute() float64 {
	return 1.0
}

func (toa *takeObjectAction) String() string {
	return fmt.Sprintf("%T[%s]", toa, toa.objectID)
}

func (toa *takeObjectAction) Estimate(memory *Memory) float64 {
	objInfo, err := memory.GetObjectInfo(toa.objectID)
	if err != nil {
		fmt.Printf("BRAIN ERROR: %s\n", err)
		return math.MaxFloat64
	}
	avgDmg := (objInfo.Attributes.SlashingDamageMax + objInfo.Attributes.SlashingDamageMin) / 2
	return (toa.finalState.EnemyPhys / avgDmg) * toa.finalState.TimeToHit
}

type equipObjectAction struct {
}

type unequipObjectAction struct {
}

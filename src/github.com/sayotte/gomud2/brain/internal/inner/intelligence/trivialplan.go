package intelligence

import (
	"fmt"
)

type trivialPlan struct {
	goalName        string
	memory          *Memory
	executionStatus int
}

func (te *trivialPlan) executeStep(msgSender MessageSender, intellect *Intellect) {
	switch te.goalName {
	case "do-nothing":
		return
	case "move-to-emptier-location":
		te.moveToAnyLocation(msgSender, intellect)
	default:
		fmt.Printf("BRAIN WARNING: don't know how to execute goal %q\n", te.goalName)
		te.executionStatus = executionPlanStatusFailed
	}
}

func (te *trivialPlan) moveToAnyLocation(msgSender MessageSender, intellect *Intellect) {
	//fmt.Println("BRAIN DEBUG: trying to move to *any* other location")

	currentZoneID, currentLocID := te.memory.GetCurrentZoneAndLocationID()
	locInfo, err := te.memory.GetLocationInfo(currentZoneID, currentLocID)
	if err != nil {
		fmt.Printf("BRAIN ERROR: %s\n", err)
		te.executionStatus = executionPlanStatusFailed
		return
	}
	for k := range locInfo.Exits {
		success, err := moveSelf(k, msgSender, intellect)
		if err != nil {
			fmt.Printf("BRAIN ERROR: %s\n", err)
			te.executionStatus = executionPlanStatusFailed
			return
		}
		if success {
			dstTuple := locInfo.Exits[k]
			dstZoneID, dstLocID := dstTuple[0], dstTuple[1]
			te.memory.SetCurrentZoneAndLocationID(dstZoneID, dstLocID)
			te.executionStatus = executionPlanStatusComplete
			return
		}
	}
}

func (te trivialPlan) status() int {
	return te.executionStatus
}

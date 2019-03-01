package intelligence

import (
	"fmt"
)

type TrivialExecutor struct{}

func (te TrivialExecutor) executeGoal(goalName string, msgSender MessageSender, intellect *Intellect, memory *Memory) {
	switch goalName {
	case "do-nothing":
		return
	case "move-to-emptier-location":
		te.moveToAnyLocation(msgSender, intellect, memory)
	default:
		fmt.Printf("BRAIN WARNING: don't know how to execute goal %q\n", goalName)
	}
}

func (te TrivialExecutor) moveToAnyLocation(msgSender MessageSender, intellect *Intellect, memory *Memory) {
	//fmt.Println("BRAIN DEBUG: trying to move to *any* other location")

	currentZoneID, currentLocID := memory.GetCurrentZoneAndLocationID()
	locInfo, err := memory.GetLocationInfo(currentZoneID, currentLocID)
	if err != nil {
		fmt.Printf("BRAIN ERROR: %s\n", err)
		return
	}
	for k := range locInfo.Exits {
		success, err := moveSelf(k, msgSender, intellect)
		if err != nil {
			fmt.Printf("BRAIN ERROR: %s\n", err)
			return
		}
		if success {
			dstTuple := locInfo.Exits[k]
			dstZoneID, dstLocID := dstTuple[0], dstTuple[1]
			memory.SetCurrentZoneAndLocationID(dstZoneID, dstLocID)
			return
		}
	}
}

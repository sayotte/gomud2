package innerbrain

import (
	"math"
)

type UtilitySelector struct {
	Selections []UtilitySelection
}

func (us UtilitySelector) selectGoal(memory *Memory) string {
	var winningSelection string
	var winningScore float64
	for _, selection := range us.Selections {
		score := selection.score(memory)
		//fmt.Printf("BRAIN DEBUG: selection: %q, score: %f\n", selection.Name, score)
		if score > winningScore {
			winningSelection = selection.Name
			winningScore = score
		}
	}
	//fmt.Printf("BRAIN DEBUG: GOAL: %q\n", winningSelection)
	return winningSelection
}

type UtilitySelection struct {
	Name           string
	Weight         float64
	Considerations []UtilityConsideration
}

func (us UtilitySelection) score(memory *Memory) float64 {
	var sum float64
	for _, cons := range us.Considerations {
		score := cons.score(memory)
		//fmt.Printf("BRAIN DEBUG: consideration: %q, score %f\n", cons.Name, score)
		sum += score
	}
	return (sum / float64(len(us.Considerations))) * us.Weight
}

type UtilityConsideration struct {
	Name        string
	CurveXParam string
	XParamRange [2]float64
	CurveType   string // currently only linear
	// y = M * (x-C)^K + B
	// so for a linear / exponential curve:
	//   M = slope
	//   K = exponent
	//   B = y-intercept
	//   C = x-intercept
	M, K, B, C float64
}

func (uc UtilityConsideration) score(memory *Memory) float64 {
	param := uc.getParam(uc.CurveXParam, memory)
	// clamp to min/max range
	if param < uc.XParamRange[0] {
		param = uc.XParamRange[0]
	} else if param > uc.XParamRange[1] {
		param = uc.XParamRange[1]
	}
	// normalize to 0.0 - 1.0
	normalized := (param - uc.XParamRange[0]) / (uc.XParamRange[1] - uc.XParamRange[0])

	return uc.M*math.Pow(normalized-uc.C, uc.K) + uc.B
}

func (uc UtilityConsideration) getParam(paramName string, memory *Memory) float64 {
	switch paramName {
	case "numActorsInLocation":
		currentZoneID, currentLocID := memory.GetCurrentZoneAndLocationID()
		return memory.GetNumActorsInLocation(currentZoneID, currentLocID) - 1.0 // -1.0 to account for ourselves
	case "secondsSinceLastMove":
		return memory.GetSecondsSinceLastMove()
	case "always-1.0":
		return 1.0
	default:
		return 0.0
	}
}

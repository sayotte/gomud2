package intelligence

func getStockUtilitySelector() UtilitySelector {
	crowdedness := UtilityConsideration{
		Name:        "too-crowded",
		CurveXParam: "numActorsInLocation",
		XParamRange: [2]float64{0.0, 8.0},
		M:           1.0,
		K:           1.0,
		B:           0.0,
		C:           0.0,
	}
	timeSpentIdling := UtilityConsideration{
		Name:        "time-since-last-move",
		CurveXParam: "secondsSinceLastMove",
		XParamRange: [2]float64{5.0, 15.0},
		M:           0.75,
		K:           1.0,
		B:           0.0,
		C:           0.0,
	}
	moveSelection := UtilitySelection{
		Name:           planGoalMoveToEmptyLocation,
		Weight:         1.0,
		Considerations: []UtilityConsideration{crowdedness, timeSpentIdling},
	}

	constantLaziness := UtilityConsideration{
		Name:        "always-be-50%-lazy",
		CurveXParam: "always-1.0",
		XParamRange: [2]float64{0.0, 1.0},
		M:           0.0,
		K:           1.0,
		B:           0.5,
		C:           0.0,
	}
	//stayIfWeapon := UtilityConsideration{
	//	Name:        "stay-in-location-with-weapon-on-ground",
	//	CurveXParam: "weaponOnGround",
	//	XParamRange: [2]float64{0.0, 1.0},
	//	M:           1.0,
	//	K:           1.0,
	//	B:           0.0,
	//	C:           0.0,
	//}
	doNothingSelection := UtilitySelection{
		Name:           planGoalDoNothing,
		Weight:         1.0,
		Considerations: []UtilityConsideration{constantLaziness}, //, stayIfWeapon},
	}

	recentlyAttacked := UtilityConsideration{
		Name:        "attacked-in-last-15-seconds",
		CurveXParam: "lastAttackedSecondsAgo",
		XParamRange: [2]float64{0.0, 17.0},
		CurveType:   "quadratic",
		M:           -1.0,
		K:           16,
		B:           1.0,
		C:           0.0,
	}
	attackerPresent := UtilityConsideration{
		Name:        "attacker-present",
		CurveXParam: "lastAttackerInLocation",
		XParamRange: [2]float64{0.0, 1.0},
		M:           1.0,
		K:           1.0,
		B:           0.0,
		C:           0.0,
	}
	defendSelfSelection := UtilitySelection{
		Name:           planGoalDefendSelf,
		Weight:         1.0,
		Considerations: []UtilityConsideration{recentlyAttacked, attackerPresent},
	}

	return UtilitySelector{
		Selections: []UtilitySelection{
			moveSelection,
			doNothingSelection,
			defendSelfSelection,
		},
	}
}

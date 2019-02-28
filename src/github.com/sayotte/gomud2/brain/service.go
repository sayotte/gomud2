package brain

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/brain/internal/innerbrain"
)

type Service struct {
	AuthUsername, AuthPassword string
	WSAPIURLString             string // "ws://localhost:4001"
}

func (s *Service) LaunchBrain(brainType string, actorID uuid.UUID) error {
	conn, err := s.initConnection()
	if err != nil {
		return err
	}

	// FIXME we should be using "brainType" to create our brain, rather than
	// FIXME statically always setting up exactly the same one

	brain := innerbrain.NewBrain(
		innerbrain.Connection{WSConn: conn},
		actorID,
		s.getHokeyUtilitySelector(),
		innerbrain.TrivialExecutor{},
	)
	return brain.Start()
}

func (s *Service) initConnection() (*websocket.Conn, error) {
	h := &http.Header{}
	userPass := s.AuthUsername + ":" + s.AuthPassword
	b64UserPass := base64.StdEncoding.EncodeToString([]byte(userPass))
	h.Add("Authorization", "Basic "+b64UserPass)

	dialer := &websocket.Dialer{}
	conn, res, err := dialer.Dial(s.WSAPIURLString, *h)
	if err != nil {
		return nil, fmt.Errorf("Dialer.Dial(%q): %s", s.WSAPIURLString, err)
	}
	if res.StatusCode != http.StatusSwitchingProtocols {
		return nil, fmt.Errorf("Dialer.Dial(): res.StatusCode: %d\n", res.StatusCode)
	}

	return conn, nil
}

func (s *Service) getHokeyUtilitySelector() innerbrain.UtilitySelector {
	crowdedness := innerbrain.UtilityConsideration{
		Name:        "too-crowded",
		CurveXParam: "numActorsInLocation",
		XParamRange: [2]float64{0.0, 8.0},
		M:           1.0,
		K:           1.0,
		B:           0.0,
		C:           0.0,
	}
	timeSpentIdling := innerbrain.UtilityConsideration{
		Name:        "time-since-last-move",
		CurveXParam: "secondsSinceLastMove",
		XParamRange: [2]float64{5.0, 15.0},
		M:           0.75,
		K:           1.0,
		B:           0.0,
		C:           0.0,
	}
	moveSelection := innerbrain.UtilitySelection{
		Name:           "move-to-emptier-location",
		Weight:         1.0,
		Considerations: []innerbrain.UtilityConsideration{crowdedness, timeSpentIdling},
	}

	constantLaziness := innerbrain.UtilityConsideration{
		Name:        "always-be-50%-lazy",
		CurveXParam: "always-1.0",
		XParamRange: [2]float64{0.0, 1.0},
		M:           0.0,
		K:           1.0,
		B:           0.5,
		C:           0.0,
	}
	stayIfWeapon := innerbrain.UtilityConsideration{
		Name:        "stay-in-location-with-weapon-on-ground",
		CurveXParam: "weaponOnGround",
		XParamRange: [2]float64{0.0, 1.0},
		M:           1.0,
		K:           1.0,
		B:           0.0,
		C:           0.0,
	}
	doNothingSelection := innerbrain.UtilitySelection{
		Name:           "do-nothing",
		Weight:         1.0,
		Considerations: []innerbrain.UtilityConsideration{constantLaziness, stayIfWeapon},
	}

	recentlyAttacked := innerbrain.UtilityConsideration{
		Name:        "attacked-in-last-15-seconds",
		CurveXParam: "lastAttackedSecondsAgo",
		XParamRange: [2]float64{0.0, 17.0},
		CurveType:   "quadratic",
		M:           -1.0,
		K:           16,
		B:           1.0,
		C:           0.0,
	}
	attackerPresent := innerbrain.UtilityConsideration{
		Name:        "attacker-present",
		CurveXParam: "lastAttackerInLocation",
		XParamRange: [2]float64{0.0, 1.0},
		M:           1.0,
		K:           1.0,
		B:           0.0,
		C:           0.0,
	}
	defendSelfSelection := innerbrain.UtilitySelection{
		Name:           "defend-self",
		Weight:         1.0,
		Considerations: []innerbrain.UtilityConsideration{recentlyAttacked, attackerPresent},
	}

	return innerbrain.UtilitySelector{
		Selections: []innerbrain.UtilitySelection{
			moveSelection,
			doNothingSelection,
			defendSelfSelection,
		},
	}
}

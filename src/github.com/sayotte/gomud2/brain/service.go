package brain

import (
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/brain/internal/inner"
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

	brain := inner.NewBrain(conn, actorID, brainType)
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

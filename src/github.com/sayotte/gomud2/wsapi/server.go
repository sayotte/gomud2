package wsapi

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/core"
	"net/http"
)

type AuthService interface {
	DoLogin(user, pass string) (uuid.UUID, *auth.AuthZDescriptor, error)
	GetActorIDsForAccountID(accountID uuid.UUID) []uuid.UUID
}

const DefaultMessageSendQueueLen = 15

type Server struct {
	ListenAddrString    string
	AuthService         AuthService
	MessageSendQueueLen int
	World               *core.World
	httpServer          *http.Server
	upgrader            *websocket.Upgrader
}

func (s *Server) Start() error {
	if s.AuthService == nil {
		return errors.New("uninitialized AuthService")
	}
	if s.World == nil {
		return errors.New("uninitialized World")
	}

	if s.MessageSendQueueLen == 0 {
		s.MessageSendQueueLen = DefaultMessageSendQueueLen
	}
	s.httpServer = &http.Server{
		Addr:    s.ListenAddrString,
		Handler: s,
	}
	err := s.httpServer.ListenAndServe()
	if err != nil {
		return fmt.Errorf("s.httpServer.ListenAndServe(): %s", err)
	}
	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	user, pass, ok := req.BasicAuth()
	if !ok {
		fmt.Println("WSAPI DEBUG: no BasicAuth creds provided")
		http.Error(w, "", http.StatusForbidden)
		return
	}
	acctID, authZDesc, err := s.AuthService.DoLogin(user, pass)
	if err != nil {
		fmt.Printf("WSAPI DEBUG: AuthService.DoLogin(): %s", err)
		http.Error(w, "", http.StatusForbidden)
		return
	}
	if !authZDesc.MayLogin {
		fmt.Printf("WSAPI DEBUG: rejecting login for user %q who is not permitted to log in", user)
		http.Error(w, "", http.StatusForbidden)
		return
	}
	fmt.Printf("WSAPI DEBUG: successful login for %q, upgrading to websocket\n", user)

	if s.upgrader == nil {
		s.upgrader = &websocket.Upgrader{}
	}
	conn, err := s.upgrader.Upgrade(w, req, http.Header{})
	if err != nil {
		http.Error(w, fmt.Sprintf("websocket.Upgrader.Upgrade(): %s", err), http.StatusInternalServerError)
		return
	}

	sess := &session{
		conn:         conn,
		authService:  s.AuthService,
		accountID:    acctID,
		authZDesc:    authZDesc,
		sendQueueLen: s.MessageSendQueueLen,
		world:        s.World,
	}
	sess.start()
}

package telnet

import (
	"errors"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/domain"
	"net"
)

type AuthService interface {
	CreateAccount(username, pass string) error
	DoLogin(user, pass string) (uuid.UUID, *auth.AuthZDescriptor, error)
	GetActorIDsForAccountID(acctID uuid.UUID) []uuid.UUID
	AddActorIDToAccountID(acctID, actorID uuid.UUID) error
}

type Server struct {
	ListenIP        string
	ListenPort      int
	MessageQueueLen int
	AuthService     AuthService
	World           *domain.World
	started         bool
	listener        *net.TCPListener
	stopChan        chan struct{}
}

func (s *Server) Start() error {
	if s.started {
		return errors.New("already started")
	}

	listenAddr := &net.TCPAddr{
		IP:   net.ParseIP(s.ListenIP),
		Port: s.ListenPort,
	}
	listener, err := net.ListenTCP("tcp", listenAddr)
	if err != nil {
		return fmt.Errorf("net.ListenTCP(): %s", err)
	}
	s.listener = listener

	s.stopChan = make(chan struct{})

	go s.acceptLoop()

	return nil
}

func (s *Server) Stop() error {
	close(s.stopChan)
	err := s.listener.Close()
	if err != nil {
		return fmt.Errorf("listener.Close(): %s", err)
	}
	return nil
}

func (s *Server) acceptLoop() {
	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		tcpConn, err := s.listener.AcceptTCP()
		if err != nil {
			fmt.Printf("DEBUG: Server.listener.AcceptTCP: %s\n", err)
			continue
		}
		fmt.Printf("DEBUG: accepted connection from %s\n", tcpConn.RemoteAddr().String())

		bufferedConn := newLineBufferedConnection(tcpConn, s.MessageQueueLen)
		err = bufferedConn.Start()
		if err != nil {
			fmt.Printf("DEBUG: Server.bufferedConn.Start: %s\n", err)
			continue
		}

		session := &session{
			authService:   s.AuthService,
			world:         s.World,
			bufferedConn:  bufferedConn,
			eventQueueLen: s.MessageQueueLen,
		}
		session.Start()
	}
}

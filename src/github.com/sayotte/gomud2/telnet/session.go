package telnet

import (
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/domain"
)

type session struct {
	lastSeenEventSequenceNumMap map[uuid.UUID]uint64

	authService   AuthService
	world         *domain.World
	eventChan     chan domain.Event
	eventQueueLen int

	terminalWidth  int
	terminalHeight int
	terminalType   string

	bufferedConn *lineBufferedConnection

	currentHandler handler

	stopChan chan struct{}
}

func (s *session) SendEvent(e domain.Event) {
	s.eventChan <- e
}

func (s *session) Start() {
	s.lastSeenEventSequenceNumMap = make(map[uuid.UUID]uint64)
	s.eventChan = make(chan domain.Event, s.eventQueueLen)
	s.stopChan = make(chan struct{})

	go s.handleLoop()
}

func (s *session) Stop() {
	close(s.stopChan)
	if s.currentHandler != nil {
		s.currentHandler.deinit()
	}
	s.bufferedConn.Shutdown()
}

func (s *session) handleLoop() {
	// initialize terminal width/height to 80x25, in case we never end up negotiating that
	s.terminalWidth = 80
	s.terminalHeight = 25

	s.currentHandler = &loginHandler{
		authService: s.authService,
		world:       s.world,
		session:     s,
	}
	outBytes := s.currentHandler.init(s.terminalWidth, s.terminalHeight)
	if len(outBytes) > 0 {
		s.bufferedConn.Send(outBytes)
	}

	for {
		select {
		case <-s.stopChan:
			return
		case e := <-s.eventChan:
			if e.SequenceNumber() <= s.lastSeenEventSequenceNumMap[e.AggregateId()] {
				continue
			}
			s.lastSeenEventSequenceNumMap[e.AggregateId()] = e.SequenceNumber()

			outBytes, newH, err := s.currentHandler.handleEvent(e, s.terminalWidth, s.terminalHeight)
			if err != nil {
				fmt.Printf("ERROR: handler.handleEvent: %s\n", err)
				s.Stop()
				return
			}
			if len(outBytes) > 0 {
				s.bufferedConn.Send(outBytes)
			}

			if newH == nil {
				fmt.Println("DEBUG: nil-handler returned, terminating connection")
				s.Stop()
				return
			}
			if newH != s.currentHandler {
				outBytes := newH.init(s.terminalWidth, s.terminalHeight)
				if len(outBytes) > 0 {
					s.bufferedConn.Send(outBytes)
				}
			}
			s.currentHandler = newH
		case newLine := <-s.bufferedConn.RxChan():
			outBytes, newH, err := s.currentHandler.handleRxLine(newLine, s.terminalWidth, s.terminalHeight)
			if err != nil {
				fmt.Printf("ERROR: handler.handleRxLine: %s\n", err)
				s.Stop()
				return
			}
			if len(outBytes) > 0 {
				s.bufferedConn.Send(outBytes)
			}

			if newH == nil {
				fmt.Println("DEBUG: nil-handler returned, terminating connection")
				s.Stop()
				return
			}
			if newH != s.currentHandler {
				outBytes := newH.init(s.terminalWidth, s.terminalHeight)
				if len(outBytes) > 0 {
					s.bufferedConn.Send(outBytes)
				}
			}
			s.currentHandler = newH
		case ctrlMsg := <-s.bufferedConn.ControlChan():
			switch ctrlMsg.messageType {
			case controlMessageTypeError:
				err := ctrlMsg.messageBody.(error)
				fmt.Printf("DEBUG: error from lineBufferedConnection: %s\n", err)
				s.Stop()
				return
			case controlMessageTypeConnectionClosed:
				s.Stop()
				fmt.Println("DEBUG: connection closed, terminating")
				return
			case controlMessageTypeTerminalType:
				s.terminalType = ctrlMsg.messageBody.(string)
				fmt.Printf("DEBUG: terminal type: %q\n", s.terminalType)
			case controlMessageTypeWindowSizeChanged:
				widthHeight := ctrlMsg.messageBody.([2]uint16)
				s.terminalWidth = int(widthHeight[0])
				s.terminalHeight = int(widthHeight[1])
				fmt.Printf("DEBUG: terminal width: %d, height: %d\n", s.terminalWidth, s.terminalHeight)
			}
		}
	}
}

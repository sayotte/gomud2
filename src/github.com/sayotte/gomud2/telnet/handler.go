package telnet

import "github.com/sayotte/gomud2/domain"

type handler interface {
	init(terminalWidth, terminalHeight int) []byte
	handleRxLine(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error)
	handleEvent(e domain.Event, terminalWidth, terminalHeight int) ([]byte, handler, error)
	deinit()
}

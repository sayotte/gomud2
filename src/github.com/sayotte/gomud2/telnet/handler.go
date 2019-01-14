package telnet

import "github.com/sayotte/gomud2/core"

type handler interface {
	init(terminalWidth, terminalHeight int) []byte
	handleRxLine(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error)
	handleEvent(e core.Event, terminalWidth, terminalHeight int) ([]byte, handler, error)
	deinit()
}

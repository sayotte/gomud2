package telnet

import (
	"fmt"
	"strconv"
)

type menu struct {
	options       []string
	lastIndexSent int
}

func (m *menu) init(terminalWidth, terminalHeight int) []byte {
	return m.printMenu(terminalWidth, terminalHeight)
}

func (m *menu) printMenu(terminalWidth, terminalHeight int) []byte {
	var retString string
	maxOptionsPerScreen := terminalHeight - 3

	if m.lastIndexSent == len(m.options) {
		// start over from the top of the menu
		m.lastIndexSent = 0
	}

	firstIdx := m.lastIndexSent
	i := 0
	for {
		if i >= maxOptionsPerScreen || firstIdx+i >= len(m.options) {
			break
		}
		retString += fmt.Sprintf("[%d] %s\n", firstIdx+i, m.options[firstIdx+i])
		i++
		m.lastIndexSent++
	}

	retString += fmt.Sprintf("\nChoose an option (0-%d)", len(m.options)-1)
	if maxOptionsPerScreen < len(m.options) {
		retString += ", or hit <enter> to display more options"
	}
	retString += "\n"

	return []byte(retString)
}

func (m *menu) handleRxLine(line []byte, terminalWidth, terminalHeight int) ([]byte, string) {
	lineS := string(line)
	if lineS == "" {
		return m.printMenu(terminalWidth, terminalHeight), ""
	}

	selection, err := strconv.Atoi(lineS)
	if err != nil {
		retS := fmt.Sprintf("Input %q doesn't appear to be an integer, try again.\n", lineS)
		return []byte(retS), ""
	}
	if selection < 0 || selection > len(m.options)-1 {
		retS := fmt.Sprintf("Invalid selection %q, try again.\n", lineS)
		return []byte(retS), ""
	}
	return nil, m.options[selection]
}

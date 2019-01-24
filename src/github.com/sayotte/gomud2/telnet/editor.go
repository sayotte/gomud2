package telnet

import (
	"errors"
	"fmt"
	"strings"

	"github.com/derekparker/trie"
	"github.com/satori/go.uuid"

	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/commands"
	"github.com/sayotte/gomud2/core"
)

const (
	worldEditHandlerStateMainMenu = iota
	worldEditHandlerStateZoneSelect
	worldEditHandlerStateZoneEdit
)

const (
	worldEditMainMenuItemEditExistingZone = "Edit an existing zone"
)


type worldEditCommandHandler func(line string, terminalWidth, terminalHeight int) ([]byte, error)

type worldEditHandler struct {
	authService AuthService
	world       *core.World
	session     *session
	accountID   uuid.UUID
	authZDesc   *auth.AuthZDescriptor

	state         int
	currentMenu   *menu
	cmdTrie       *trie.Trie
	zoneUnderEdit *core.Zone
	locUnderEdit  *core.Location
}

func (weh *worldEditHandler) init(terminalWidth, terminalHeight int) []byte {
	return weh.gotoMainMenu(terminalWidth, terminalHeight)
}

func (weh *worldEditHandler) handleRxLine(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	switch weh.state {
	case worldEditHandlerStateMainMenu:
		return weh.handleMainMenuState(line, terminalWidth, terminalHeight)
	case worldEditHandlerStateZoneSelect:
		return weh.handleZoneSelectState(line, terminalWidth, terminalHeight)
	case worldEditHandlerStateZoneEdit:
		return weh.handleZoneEditState(line, terminalWidth, terminalHeight)
	default:
		return nil, weh, fmt.Errorf("worldEditHandler: unknown state %d", weh.state)
	}
}

func (weh *worldEditHandler) handleEvent(e core.Event, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	return nil, weh, nil
}

func (weh *worldEditHandler) deinit() {

}

func (weh *worldEditHandler) gotoMainMenu(terminalWidth, terminalHeight int) []byte {
	weh.state = worldEditHandlerStateMainMenu

	var options []string
	options = append(options, worldEditMainMenuItemEditExistingZone)
	options = append(options, menuItemCancel)

	weh.currentMenu = &menu{
		options: options,
	}
	return weh.currentMenu.init(terminalWidth, terminalHeight)
}

func (weh *worldEditHandler) handleMainMenuState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	outBytes, selection := weh.currentMenu.handleRxLine(line, terminalWidth, terminalHeight)
	if selection == "" {
		return outBytes, weh, nil
	}

	switch selection {
	case worldEditMainMenuItemEditExistingZone:
		return weh.gotoZoneSelectState(terminalWidth, terminalHeight), weh, nil
	case menuItemCancel:
		lobbyHandler := &lobbyHandler{
			authService: weh.authService,
			world:       weh.world,
			session:     weh.session,
			accountID:   weh.accountID,
			authZDesc:   weh.authZDesc,
		}
		return nil, lobbyHandler, nil
	default:
		return []byte("That's nice, dear.\n"), weh, nil
	}
}

func (weh *worldEditHandler) gotoZoneSelectState(terminalWidth, terminalHeight int) []byte {
	weh.state = worldEditHandlerStateZoneSelect

	var options []string
	for _, zone := range weh.world.Zones() {
		options = append(options, zone.Tag())
	}
	options = append(options, menuItemCancel)

	weh.currentMenu = &menu{
		options: options,
	}
	return weh.currentMenu.init(terminalWidth, terminalHeight)
}

func (weh *worldEditHandler) handleZoneSelectState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	outBytes, selection := weh.currentMenu.handleRxLine(line, terminalWidth, terminalHeight)
	if selection == "" {
		return outBytes, weh, nil
	}

	switch selection {
	case menuItemCancel:
		return weh.gotoMainMenu(terminalWidth, terminalHeight), weh, nil
	default:
		zoneTagParts := strings.Split(string(selection), "/")
		if len(zoneTagParts) < 2 {
			return []byte("Whoops..."), weh, fmt.Errorf("malformed zone tag: %q", line)
		}
		zoneID, err := uuid.FromString(zoneTagParts[1])
		if err != nil {
			return []byte("Whoops..."), weh, fmt.Errorf("uuid.FromString(%q): %s", zoneTagParts[1], err)
		}
		zone := weh.world.ZoneByID(zoneID)
		if zone == nil {
			return []byte("Whoops..."), weh, fmt.Errorf("no such zone %q", zoneID)
		}
		return weh.gotoZoneEditState(terminalWidth, zone, nil), weh, nil
	}
}

func (weh *worldEditHandler) gotoZoneEditState(terminalWidth int, zone *core.Zone, loc *core.Location) []byte {
	weh.state = worldEditHandlerStateZoneEdit
	weh.zoneUnderEdit = zone
	if loc == nil {
		weh.locUnderEdit = zone.Locations()[0]
	}

	weh.cmdTrie = trie.New()
	weh.cmdTrie.Add(core.EdgeDirectionNorth, weh.getDirectionHandlerGeneric(core.EdgeDirectionNorth))
	weh.cmdTrie.Add(core.EdgeDirectionSouth, weh.getDirectionHandlerGeneric(core.EdgeDirectionSouth))
	weh.cmdTrie.Add(core.EdgeDirectionEast, weh.getDirectionHandlerGeneric(core.EdgeDirectionEast))
	weh.cmdTrie.Add(core.EdgeDirectionWest, weh.getDirectionHandlerGeneric(core.EdgeDirectionWest))
	weh.cmdTrie.Add("exit", weh.getExitHandler())
	weh.cmdTrie.Add("look", weh.getLookHandler())
	weh.cmdTrie.Add("commands", weh.getCommandsHandler())

	return lookAtLocation(nil, terminalWidth, weh.locUnderEdit)
}

func (weh *worldEditHandler) handleZoneEditState(lineB []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	line := string(lineB)
	if line == "" {
		return nil, weh, nil
	}

	firstTerm := strings.ToLower(strings.Split(line, " ")[0])
	terms := weh.cmdTrie.PrefixSearch(firstTerm)
	if len(terms) == 0 {
		return []byte(fmt.Sprintf("Unrecognized command %q, try \"commands\".\n", firstTerm)), weh, nil
	}

	restOfLine := strings.TrimLeft(strings.TrimPrefix(line, firstTerm), " ")

	node, _ := weh.cmdTrie.Find(terms[0])
	cmdHandler, ok := node.Meta().(worldEditCommandHandler)
	if !ok {
		return []byte(fmt.Sprintf("Registered but unimplemented command: %q, rest of line: %q\n", terms[0], restOfLine)), weh, nil
	}
	outBytes, err := cmdHandler(restOfLine, terminalWidth, terminalHeight)
	return outBytes, weh, err
}

func (weh *worldEditHandler) getLookHandler() worldEditCommandHandler {
	return func(line string, terminalWidth, terminalHeight int) ([]byte, error) {
		return lookAtLocation(nil, terminalWidth, weh.locUnderEdit), nil
	}
}

func (weh *worldEditHandler) getCommandsHandler() worldEditCommandHandler {
	return func(line string, terminalWidth, terminalHeight int) ([]byte, error) {
		return summarizeCommands(weh.cmdTrie, terminalWidth), nil
	}
}

func (weh *worldEditHandler) getDirectionHandlerGeneric(direction string) worldEditCommandHandler {
	return func(line string, terminalWidth, terminalHeight int) ([]byte, error) {
		var outEdge *core.LocationEdge
		for _, edge := range weh.locUnderEdit.OutEdges() {
			if edge.Direction() == direction {
				outEdge = edge
				break
			}
		}
		if outEdge == nil {
			return []byte(commands.ErrorNoSuchExit), nil
		}

		var outBytes []byte
		if outEdge.Destination() == nil {
			destZone := weh.world.ZoneByID(outEdge.OtherZoneID())
			if destZone == nil {
				return []byte(fmt.Sprintf("Link to Zone %q, not currently loaded in World.\n", outEdge.OtherZoneID())), nil
			}
			destLoc := destZone.LocationByID(outEdge.OtherZoneLocID())
			if destLoc == nil {
				return []byte(fmt.Sprintf("Link to Location %q, in remote Zone %q-- location not found in remote Zone.\n", outEdge.OtherZoneLocID(), destZone.Tag())), nil
			}
			outBytes = []byte(fmt.Sprintf("WARNING!!! Moving to different zone %q\n", destZone.Tag()))
			weh.zoneUnderEdit = destZone
			weh.locUnderEdit = destLoc
		} else {
			weh.locUnderEdit = outEdge.Destination()
		}

		return append(outBytes, lookAtLocation(nil, terminalWidth, weh.locUnderEdit)...), nil
	}
}

func (weh *worldEditHandler) getExitHandler() worldEditCommandHandler {
	return func(ignoredS string, terminalWidth, terminalHeight int) ([]byte, error) {
		return weh.gotoMainMenu(terminalWidth, terminalHeight), nil
	}
}


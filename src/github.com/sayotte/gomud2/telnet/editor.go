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
	worldEditHandlerStateLocationEdit
	worldEditHandlerStateLocationGetShortDesc
	worldEditHandlerStateLocationGetDesc
	worldEditHandlerStateExitEdit
	worldEditHandlerStateExitGetDescription
	worldEditHandlerStateExitGetDirection
	worldEditHandlerStateExitGetDestination
)

const (
	worldEditMainMenuItemEditExistingZone = "Edit an existing zone"
)

const (
	worldEditLocEditMenuItemShortDescription = "Change short description"
	worldEditLocEditMenuItemLongDescription  = "Change long description"
)

const (
	worldEditExitEditMenuItemDescription = "Change description"
	worldEditExitEditMenuItemDirection   = "Change direction"
	worldEditExitEditMenuItemDest        = "Change destination"
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
	exitUnderEdit *core.Exit
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
	case worldEditHandlerStateLocationEdit:
		return weh.handleEditLocationState(line, terminalWidth, terminalHeight)
	case worldEditHandlerStateLocationGetShortDesc:
		return weh.handleGetLocationShortDescState(line, terminalWidth, terminalHeight)
	case worldEditHandlerStateLocationGetDesc:
		return weh.handleGetLocationDescState(line, terminalWidth, terminalHeight)
	case worldEditHandlerStateExitEdit:
		return weh.handleEditExitState(line, terminalWidth, terminalHeight)
	case worldEditHandlerStateExitGetDescription:
		return weh.handleGetExitDescState(line, terminalWidth, terminalHeight)
	case worldEditHandlerStateExitGetDirection:
		return weh.handleGetExitDirectionState(line, terminalWidth, terminalHeight)
	case worldEditHandlerStateExitGetDestination:
		return weh.handleGetExitDestState(line, terminalWidth, terminalHeight)
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
	weh.cmdTrie.Add(core.ExitDirectionNorth, weh.getDirectionHandlerGeneric(core.ExitDirectionNorth))
	weh.cmdTrie.Add(core.ExitDirectionSouth, weh.getDirectionHandlerGeneric(core.ExitDirectionSouth))
	weh.cmdTrie.Add(core.ExitDirectionEast, weh.getDirectionHandlerGeneric(core.ExitDirectionEast))
	weh.cmdTrie.Add(core.ExitDirectionWest, weh.getDirectionHandlerGeneric(core.ExitDirectionWest))
	weh.cmdTrie.Add("goto", weh.getGotoHandler())
	weh.cmdTrie.Add("leave", weh.getLeaveHandler())
	weh.cmdTrie.Add("look", weh.getLookHandler())
	weh.cmdTrie.Add("inspect", weh.getInspectHandler())
	weh.cmdTrie.Add("newlocation", weh.getNewlocationHandler())
	weh.cmdTrie.Add("editlocation", weh.gotoEditLocationMenu())
	weh.cmdTrie.Add("orphanlocations", weh.getOrphanLocationsHandler())
	weh.cmdTrie.Add("editexit", weh.getEditExitHandler())
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

func (weh *worldEditHandler) getGotoHandler() worldEditCommandHandler {
	return func(line string, terminalWidth, terminalHeight int) ([]byte, error) {
		params := strings.Split(line, " ")
		if len(params) == 0 {
			return []byte("Goto where? Need the ID of a Location.\n"), nil
		}

		locID, err := uuid.FromString(params[0])
		if err != nil {
			return []byte(fmt.Sprintf("Invalid Location ID: %s\n", err)), nil
		}

		loc := weh.zoneUnderEdit.LocationByID(locID)
		if loc == nil {
			return []byte(fmt.Sprintf("No such Location %q in this Zone.\n", locID)), nil
		}

		weh.locUnderEdit = loc
		return lookAtLocation(nil, terminalWidth, weh.locUnderEdit), nil
	}
}

func (weh *worldEditHandler) getOrphanLocationsHandler() worldEditCommandHandler {
	return func(line string, terminalWidth, terminalHeight int) ([]byte, error) {
		locHasIncomingExits := make(map[*core.Location]bool)
		for _, exit := range weh.zoneUnderEdit.Exits() {
			if exit.Destination() != nil {
				locHasIncomingExits[exit.Destination()] = true
			}
		}

		var orphanLocTags []string
		for _, loc := range weh.zoneUnderEdit.Locations() {
			if !locHasIncomingExits[loc] {
				orphanLocTags = append(orphanLocTags, loc.Tag())
			}
		}

		summaryFmt := "Orphaned Locations (having no incoming exits from other Locations in this Zone):\n%s\n"
		return []byte(fmt.Sprintf(summaryFmt, strings.Join(orphanLocTags, "\n"))), nil
	}
}

func (weh *worldEditHandler) getDirectionHandlerGeneric(direction string) worldEditCommandHandler {
	return func(line string, terminalWidth, terminalHeight int) ([]byte, error) {
		var outExit *core.Exit
		for _, exit := range weh.locUnderEdit.OutExits() {
			if exit.Direction() == direction {
				outExit = exit
				break
			}
		}
		if outExit == nil {
			return []byte(commands.ErrorNoSuchExit), nil
		}

		var outBytes []byte
		if outExit.Destination() == nil {
			destZone := weh.world.ZoneByID(outExit.OtherZoneID())
			if destZone == nil {
				return []byte(fmt.Sprintf("Link to Zone %q, not currently loaded in World.\n", outExit.OtherZoneID())), nil
			}
			destLoc := destZone.LocationByID(outExit.OtherZoneLocID())
			if destLoc == nil {
				return []byte(fmt.Sprintf("Link to Location %q, in remote Zone %q-- location not found in remote Zone.\n", outExit.OtherZoneLocID(), destZone.Tag())), nil
			}
			outBytes = []byte(fmt.Sprintf("WARNING!!! Moving to different zone %q\n", destZone.Tag()))
			weh.zoneUnderEdit = destZone
			weh.locUnderEdit = destLoc
		} else {
			weh.locUnderEdit = outExit.Destination()
		}

		return append(outBytes, lookAtLocation(nil, terminalWidth, weh.locUnderEdit)...), nil
	}
}

func (weh *worldEditHandler) getLeaveHandler() worldEditCommandHandler {
	return func(ignoredS string, terminalWidth, terminalHeight int) ([]byte, error) {
		return weh.gotoMainMenu(terminalWidth, terminalHeight), nil
	}
}

func (weh *worldEditHandler) getInspectHandler() worldEditCommandHandler {
	return func(line string, terminalWidth, terminalHeight int) ([]byte, error) {
		params := strings.Split(line, " ")
		if len(params) == 0 {
			return []byte("Inspect what? " + inspectNoSubcmdErr), nil
		}
		subcmd := strings.ToLower(params[0])
		switch subcmd {
		case inspectSubcmdLocation:
			ilr := &inspectLocationReport{}
			ilr.fromLocation(weh.locUnderEdit)
			return ilr.bytes(), nil
		case inspectSubcmdExits:
			ioer := inspectOutExitsReport{location: weh.locUnderEdit}
			return ioer.bytes(), nil
		default:
			return []byte(fmt.Sprintf("Don't know how to inspect %q. %s", subcmd, inspectNoSubcmdErr)), nil
		}
	}
}

func (weh *worldEditHandler) getNewlocationHandler() worldEditCommandHandler {
	return func(line string, terminalWidth, terminalHeight int) ([]byte, error) {
		params := strings.Split(line, " ")
		if len(params) == 0 {
			return []byte("Usage: newlocation <direction>\n"), nil
		}
		direction := strings.ToLower(params[0])
		if !core.ValidDirections[direction] {
			return []byte(fmt.Sprintf("Invalid direction %q, need one of: %s\n", direction, strings.Join(orderedDirections, ", "))), nil
		}

		for _, exit := range weh.locUnderEdit.OutExits() {
			if exit.Direction() == direction {
				return []byte("Out-exit in that direction already exists from this Location.\n"), nil
			}
		}

		shortDesc := "Short description goes here"
		longDesc := "Long description goes here"
		newLocPrim := core.NewLocation(uuid.Nil, weh.zoneUnderEdit, shortDesc, longDesc)
		newLoc, err := weh.zoneUnderEdit.AddLocation(newLocPrim)
		if err != nil {
			return []byte(fmt.Sprintf("ERROR: AddLocation(): %s\n", err)), nil
		}

		outExitPrim := core.NewExit(
			uuid.Nil,
			fmt.Sprintf("To %s", newLoc.ID()),
			direction,
			weh.locUnderEdit,
			newLoc,
			weh.zoneUnderEdit,
			uuid.Nil,
			uuid.Nil,
		)
		_, err = weh.zoneUnderEdit.AddExit(outExitPrim)
		if err != nil {
			return []byte(fmt.Sprintf("ERROR: AddExit(1): %s\n", err)), nil
		}

		inExitPrim := core.NewExit(
			uuid.Nil,
			fmt.Sprintf("To %s", weh.locUnderEdit.ID()),
			invertDirection(direction),
			newLoc,
			weh.locUnderEdit,
			weh.zoneUnderEdit,
			uuid.Nil,
			uuid.Nil,
		)
		_, err = weh.zoneUnderEdit.AddExit(inExitPrim)
		if err != nil {
			return []byte(fmt.Sprintf("ERROR: AddExit(2): %s\n", err)), nil
		}

		return []byte("Done.\n"), nil
	}
}

func (weh *worldEditHandler) gotoEditLocationMenu() worldEditCommandHandler {
	return func(line string, terminalWidth, terminalHeight int) ([]byte, error) {
		ilr := &inspectLocationReport{}
		ilr.fromLocation(weh.locUnderEdit)
		locInspectBytes := ilr.bytes()

		weh.state = worldEditHandlerStateLocationEdit
		options := []string{
			worldEditLocEditMenuItemShortDescription,
			worldEditLocEditMenuItemLongDescription,
			menuItemCancel,
		}
		weh.currentMenu = &menu{
			options: options,
		}

		outBytes := append(locInspectBytes, weh.currentMenu.init(terminalWidth, terminalHeight)...)
		return outBytes, nil
	}
}

func (weh *worldEditHandler) handleEditLocationState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	outBytes, selection := weh.currentMenu.handleRxLine(line, terminalWidth, terminalHeight)
	if selection == "" {
		return outBytes, weh, nil
	}

	switch selection {
	case worldEditLocEditMenuItemShortDescription:
		weh.state = worldEditHandlerStateLocationGetShortDesc
		return []byte("Enter new short description, followed by a newline <enter>.\n"), weh, nil
	case worldEditLocEditMenuItemLongDescription:
		weh.state = worldEditHandlerStateLocationGetDesc
		return []byte("Enter new description, followed by a newline <enter>\n"), weh, nil
	case menuItemCancel:
		fallthrough
	default:
		outBytes := weh.gotoZoneEditState(terminalWidth, weh.zoneUnderEdit, weh.locUnderEdit)
		return outBytes, weh, nil
	}
}

func (weh *worldEditHandler) handleGetLocationShortDescState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	newShortDesc := strings.TrimSuffix(string(line), "\n")
	err := weh.locUnderEdit.Update(
		newShortDesc,
		weh.locUnderEdit.Description(),
	)
	if err != nil {
		fmt.Printf("ERROR: Location.Update(...): %s\n", err)
		return nil, weh, errors.New("Whoops...")
	}

	gotoMenuFunc := weh.gotoEditLocationMenu()
	menuBytes, _ := gotoMenuFunc(string(line), terminalWidth, terminalHeight)
	return append([]byte("Done.\n"), menuBytes...), weh, nil
}

func (weh *worldEditHandler) handleGetLocationDescState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	newDesc := strings.TrimSuffix(string(line), "\n")
	err := weh.locUnderEdit.Update(
		weh.locUnderEdit.ShortDescription(),
		newDesc,
	)
	if err != nil {
		fmt.Printf("ERROR: Location.Update(...): %s\n", err)
		return nil, weh, errors.New("Whoops...")
	}

	gotoMenuFunc := weh.gotoEditLocationMenu()
	menuBytes, _ := gotoMenuFunc(string(line), terminalWidth, terminalHeight)
	return append([]byte("Done.\n"), menuBytes...), weh, nil
}

func (weh *worldEditHandler) getEditExitHandler() worldEditCommandHandler {
	return func(line string, terminalWidth, terminalHeight int) ([]byte, error) {
		params := strings.Split(line, " ")
		if len(params) == 0 {
			return []byte("Usage: editexit <direction>\n"), nil
		}
		direction := strings.ToLower(params[0])
		if !core.ValidDirections[direction] {
			return []byte(fmt.Sprintf("Invalid direction %q, need one of: %s\n", direction, strings.Join(orderedDirections, ", "))), nil
		}

		var exit *core.Exit
		for _, maybeExit := range weh.locUnderEdit.OutExits() {
			if maybeExit.Direction() == direction {
				exit = maybeExit
				break
			}
		}
		if exit == nil {
			return []byte(fmt.Sprintf("No exit in direction %q.\n", direction)), nil
		}

		return weh.gotoEditExitState(exit, terminalWidth, terminalHeight), nil
	}
}

func (weh *worldEditHandler) gotoEditExitState(exit *core.Exit, terminalWidth, terminalHeight int) []byte {
	weh.state = worldEditHandlerStateExitEdit
	weh.exitUnderEdit = exit

	iler := &inspectExitReport{}
	iler.fromExit(weh.exitUnderEdit)

	options := []string{
		worldEditExitEditMenuItemDescription,
		worldEditExitEditMenuItemDirection,
		worldEditExitEditMenuItemDest,
		menuItemCancel,
	}
	weh.currentMenu = &menu{
		options: options,
	}
	return append(iler.bytes(), weh.currentMenu.init(terminalWidth, terminalHeight)...)
}

func (weh *worldEditHandler) handleEditExitState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	outBytes, selection := weh.currentMenu.handleRxLine(line, terminalWidth, terminalHeight)
	if selection == "" {
		return outBytes, weh, nil
	}

	switch selection {
	case worldEditExitEditMenuItemDescription:
		weh.state = worldEditHandlerStateExitGetDescription
		return []byte("Enter new description, followed by a newline <enter>.\n"), weh, nil
	case worldEditExitEditMenuItemDirection:
		weh.state = worldEditHandlerStateExitGetDirection
		return []byte(fmt.Sprintf("Enter one of [%s] followed by a newline <enter>.\n", strings.Join(orderedDirections, ", "))), weh, nil
	case worldEditExitEditMenuItemDest:
		weh.state = worldEditHandlerStateExitGetDestination
		prompt := "Enter ID for destination Zone/Location, followed by a newline <enter>\n"
		prompt += "Example: b6b0fff7-a7fe-4ba2-91e0-9e78e752f841/3730ad94-88fa-4f11-8cbc-bcebdaa0ac9b\n"
		return []byte(prompt), weh, nil
	case menuItemCancel:
		fallthrough
	default:
		outBytes := weh.gotoZoneEditState(terminalWidth, weh.zoneUnderEdit, weh.locUnderEdit)
		return outBytes, weh, nil
	}
}

func (weh *worldEditHandler) handleGetExitDescState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	newDesc := strings.TrimSuffix(string(line), "\n")

	err := weh.exitUnderEdit.Update(
		newDesc,
		weh.exitUnderEdit.Direction(),
		weh.exitUnderEdit.Source(),
		weh.exitUnderEdit.Destination(),
		weh.exitUnderEdit.OtherZoneID(),
		weh.exitUnderEdit.OtherZoneLocID(),
	)
	if err != nil {
		fmt.Printf("ERROR: Exit.Update(description): %s\n", err)
		return nil, weh, errors.New("Whoops...")
	}

	menuBytes := weh.gotoEditExitState(weh.exitUnderEdit, terminalWidth, terminalHeight)
	return append([]byte("Done.\n"), menuBytes...), weh, nil
}

func (weh *worldEditHandler) handleGetExitDirectionState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	newDir := strings.TrimSuffix(string(line), "\n")
	if !core.ValidDirections[newDir] {
		errBytes := []byte(fmt.Sprintf("Invalid direction, must be one of [%s].\n", strings.Join(orderedDirections, ", ")))
		menuBytes := weh.gotoEditExitState(weh.exitUnderEdit, terminalWidth, terminalHeight)
		return append(errBytes, menuBytes...), weh, nil
	}
	for _, existingExit := range weh.locUnderEdit.OutExits() {
		if existingExit.Direction() == newDir {
			errBytes := []byte("There's an existing exit in that direction!\n")
			menuBytes := weh.gotoEditExitState(weh.exitUnderEdit, terminalWidth, terminalHeight)
			return append(errBytes, menuBytes...), weh, nil
		}
	}

	err := weh.exitUnderEdit.Update(
		weh.exitUnderEdit.Description(),
		newDir,
		weh.exitUnderEdit.Source(),
		weh.exitUnderEdit.Destination(),
		weh.exitUnderEdit.OtherZoneID(),
		weh.exitUnderEdit.OtherZoneLocID(),
	)
	if err != nil {
		fmt.Printf("ERROR: Exit.Update(destination): %s\n", err)
		return nil, weh, errors.New("Whoops...")
	}
	menuBytes := weh.gotoEditExitState(weh.exitUnderEdit, terminalWidth, terminalHeight)
	return append([]byte("Done.\n"), menuBytes...), weh, nil
}

func (weh *worldEditHandler) handleGetExitDestState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	in := strings.TrimSuffix(string(line), "\n")
	parts := strings.Split(in, "/")
	if len(parts) != 2 {
		errBytes := []byte("Invalid input, must be of form <ID>/<ID>\n")
		return append(errBytes, weh.gotoEditExitState(weh.exitUnderEdit, terminalWidth, terminalHeight)...), weh, nil
	}

	zoneID, err := uuid.FromString(parts[0])
	if err != nil {
		errBytes := []byte(fmt.Sprintf("Invalid Zone ID: %s\n", err))
		return append(errBytes, weh.gotoEditExitState(weh.exitUnderEdit, terminalWidth, terminalHeight)...), weh, nil
	}
	locID, err := uuid.FromString(parts[1])
	if err != nil {
		errBytes := []byte(fmt.Sprintf("Invalid Location ID: %s\n", err))
		return append(errBytes, weh.gotoEditExitState(weh.exitUnderEdit, terminalWidth, terminalHeight)...), weh, nil
	}

	if uuid.Equal(zoneID, weh.exitUnderEdit.Zone().ID()) {
		// Handle both of these cases:
		// - internal -> internal
		// - external -> internal
		dest := weh.exitUnderEdit.Zone().LocationByID(locID)
		if dest == nil {
			errBytes := []byte(fmt.Sprintf("No such Location with ID %q in this Zone\n", locID))
			return append(errBytes, weh.gotoEditExitState(weh.exitUnderEdit, terminalWidth, terminalHeight)...), weh, nil
		}
		err := weh.exitUnderEdit.Update(
			weh.exitUnderEdit.Description(),
			weh.exitUnderEdit.Direction(),
			weh.exitUnderEdit.Source(),
			dest,
			uuid.Nil,
			uuid.Nil,
		)
		if err != nil {
			fmt.Printf("ERROR: Exit.Update(destination): %s\n", err)
			return nil, weh, errors.New("Whoops...")
		}
	} else {
		// Handle both of these cases:
		// internal -> external
		// external -> external
		err := weh.exitUnderEdit.Update(
			weh.exitUnderEdit.Description(),
			weh.exitUnderEdit.Direction(),
			weh.exitUnderEdit.Source(),
			nil,
			zoneID,
			locID,
		)
		if err != nil {
			fmt.Printf("ERROR: Exit.Update(destination): %s\n", err)
			return nil, weh, errors.New("Whoops...")
		}
	}
	return append([]byte("Done.\n"), weh.gotoEditExitState(weh.exitUnderEdit, terminalWidth, terminalHeight)...), weh, nil
}

func invertDirection(inDir string) string {
	switch inDir {
	case core.ExitDirectionNorth:
		return core.ExitDirectionSouth
	case core.ExitDirectionSouth:
		return core.ExitDirectionNorth
	case core.ExitDirectionEast:
		return core.ExitDirectionWest
	case core.ExitDirectionWest:
		return core.ExitDirectionEast
	default:
		return ""
	}
}

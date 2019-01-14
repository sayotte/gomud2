package telnet

import (
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/core"
)

const (
	lobbyHandlerStateMainMenu = iota
	lobbyHandlerStateGetCharacterName
	lobbyHandlerStateSelectExistingActor
	lobbyHandlerStateOperationsMenu
)

const (
	mainMenuItemPossessCharacter = "Take control of an existing character"
	mainMenuItemCreateCharacter  = "Create a new character"
	mainMenuItemAccountSettings  = "Account settings and maintenance"
	mainMenuItemServerOperations = "MUD server operations"
	mainMenuItemDisconnect       = "Disconnect"
	menuItemCancel               = "Cancel"
	opsMenuItemSnapshot          = "Store a snapshot of current MUD state"
)

type lobbyHandler struct {
	authService  AuthService
	world        *core.World
	session      *session
	accountID    uuid.UUID
	authZDesc    *auth.AuthZDescriptor
	currentMenu  *menu
	state        int
	actorsByName map[string]*core.Actor
}

func (lh *lobbyHandler) init(terminalWidth, terminalHeight int) []byte {
	return lh.gotoMainMenu(terminalWidth, terminalHeight)
}

func (lh *lobbyHandler) handleEvent(e core.Event, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	return nil, lh, nil
}

func (lh *lobbyHandler) handleRxLine(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	switch lh.state {
	case lobbyHandlerStateMainMenu:
		return lh.handleMainMenuState(line, terminalWidth, terminalHeight)
	case lobbyHandlerStateGetCharacterName:
		return lh.handleGetCharacterNameState(line)
	case lobbyHandlerStateSelectExistingActor:
		return lh.handleActorSelectState(line, terminalWidth, terminalHeight)
	case lobbyHandlerStateOperationsMenu:
		outBytes, err := lh.handleOperationsMenuState(line, terminalWidth, terminalHeight)
		return outBytes, lh, err
	default:
		return []byte("That's nice, dear.\n"), lh, nil
	}
}

func (lh *lobbyHandler) gotoMainMenu(terminalWidth, terminalHeight int) []byte {
	lh.state = lobbyHandlerStateMainMenu
	var options []string

	if lh.authZDesc.CreateCharacter {
		options = append(options, mainMenuItemCreateCharacter)
	}
	if lh.authZDesc.PossessCharacter {
		options = append(options, mainMenuItemPossessCharacter)
	}
	if lh.authZDesc.ServerOperations {
		options = append(options, mainMenuItemServerOperations)
	}
	options = append(options, mainMenuItemAccountSettings, mainMenuItemDisconnect)

	lh.currentMenu = &menu{
		options: options,
	}

	return lh.currentMenu.init(terminalWidth, terminalHeight)
}

func (lh *lobbyHandler) handleMainMenuState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	outBytes, selection := lh.currentMenu.handleRxLine(line, terminalWidth, terminalHeight)
	if selection == "" {
		return outBytes, lh, nil
	}

	switch selection {
	case mainMenuItemCreateCharacter:
		lh.state = lobbyHandlerStateGetCharacterName
		return []byte("Character name?: "), lh, nil
	case mainMenuItemPossessCharacter:
		outBytes := lh.gotoActorSelectMenu(terminalWidth, terminalHeight)
		return outBytes, lh, nil
	case mainMenuItemServerOperations:
		outBytes := lh.gotoOperationsMenu(terminalWidth, terminalHeight)
		return outBytes, lh, nil
	case mainMenuItemDisconnect:
		return []byte("Ok, bye!\n"), nil, nil
	default:
		return nil, lh, nil
	}
}

func (lh *lobbyHandler) handleGetCharacterNameState(line []byte) ([]byte, handler, error) {
	name := string(line)
	if name == "" {
		return []byte("We need a non-empty name, try again.\nCharacter name?: "), lh, nil
	}

	actorPre := core.NewActor(uuid.Nil, name, nil, nil)
	actor, err := lh.world.AddActor(actorPre)
	if err != nil {
		return nil, nil, fmt.Errorf("world.AddActor: %s", err)
	}
	err = lh.authService.AddActorIDToAccountID(lh.accountID, actor.ID())
	if err != nil {
		return nil, nil, fmt.Errorf("authService.AddActorIDtoAccountID(): %s", err)
	}

	return lh.gotoGameHandler(actor)
}

func (lh *lobbyHandler) gotoActorSelectMenu(terminalWidth, terminalHeight int) []byte {
	lh.state = lobbyHandlerStateSelectExistingActor
	lh.actorsByName = make(map[string]*core.Actor)
	var menuOptions []string
	actorIDs := lh.authService.GetActorIDsForAccountID(lh.accountID)
	for _, actorID := range actorIDs {
		a := lh.world.ActorByID(actorID)
		if a != nil {
			nameString := fmt.Sprintf("%s/%s", a.Name(), a.ID())
			lh.actorsByName[nameString] = a
			menuOptions = append(menuOptions, nameString)
		}
	}

	menuOptions = append(menuOptions, menuItemCancel)
	lh.currentMenu = &menu{
		options: menuOptions,
	}
	return lh.currentMenu.init(terminalWidth, terminalHeight)
}

func (lh *lobbyHandler) handleActorSelectState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	outBytes, selection := lh.currentMenu.handleRxLine(line, terminalWidth, terminalHeight)
	if selection == "" {
		return outBytes, lh, nil
	}

	if selection == menuItemCancel {
		outBytes = lh.gotoMainMenu(terminalWidth, terminalHeight)
		return outBytes, lh, nil
	}

	selectedActor, found := lh.actorsByName[selection]
	if !found {
		return nil, nil, fmt.Errorf("user somehow provided actor-selection string %q not found in menu", selection)
	}

	return lh.gotoGameHandler(selectedActor)
}

func (lh *lobbyHandler) gotoGameHandler(actor *core.Actor) ([]byte, handler, error) {
	actor.AddObserver(lh.session)
	gameHandler := &gameHandler{
		authZDesc: lh.authZDesc,
		session:   lh.session,
		actor:     actor,
		world:     lh.world,
	}

	return []byte("Entering world...\n"), gameHandler, nil
}

func (lh *lobbyHandler) gotoOperationsMenu(terminalWidth, terminalHeight int) []byte {
	lh.state = lobbyHandlerStateOperationsMenu
	menuOptions := []string{
		opsMenuItemSnapshot,
		menuItemCancel,
	}
	lh.currentMenu = &menu{
		options: menuOptions,
	}
	return lh.currentMenu.init(terminalWidth, terminalHeight)
}

func (lh *lobbyHandler) handleOperationsMenuState(line []byte, terminalWidth, terminalHeight int) ([]byte, error) {
	outBytes, selection := lh.currentMenu.handleRxLine(line, terminalWidth, terminalHeight)
	if selection == "" {
		return outBytes, nil
	}

	switch selection {
	case opsMenuItemSnapshot:
		err := lh.world.Snapshot()
		if err != nil {
			return []byte("Error! See server log for details.\n"), err
		}
		outBytes := []byte("Success.\n")
		outBytes = append(outBytes, lh.gotoOperationsMenu(terminalWidth, terminalHeight)...)
		return outBytes, nil
	case menuItemCancel:
		outBytes := lh.gotoMainMenu(terminalWidth, terminalHeight)
		return outBytes, nil
	}

	return nil, nil
}

func (lh *lobbyHandler) deinit() {
	return
}

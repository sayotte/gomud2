package telnet

import (
	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/domain"
)

const (
	loginHandlerStateInit = iota
	loginHandlerStateFirstMenu
	loginHandlerStateGetLoginUser
	loginHandlerStateGetLoginPass
	loginHandlerStateGetNewUser
	loginHandlerStateGetNewPass
)

const (
	loginMenuItemLogin         = "Login"
	loginMenuItemCreateAccount = "Create account"
	loginMenuItemDisconnect    = "Disconnect"
)

type loginHandler struct {
	authService AuthService
	world       *domain.World
	session     *session
	currentMenu *menu
	state       int
	username    string
}

func (lh *loginHandler) init(terminalWidth, terminalHeight int) []byte {
	return []byte("Pausing here to probe your terminal/client. Please hit <enter> to continue.\n")
}

func (lh *loginHandler) handleEvent(e domain.Event, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	return nil, lh, nil
}

func (lh *loginHandler) handleRxLine(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	switch lh.state {
	case loginHandlerStateInit:
		return lh.handleLineInitState(terminalWidth, terminalHeight), lh, nil
	case loginHandlerStateFirstMenu:
		return lh.handleLineFirstMenuState(line, terminalWidth, terminalHeight)
	case loginHandlerStateGetLoginUser:
		return lh.handleGetLoginUserState(line)
	case loginHandlerStateGetLoginPass:
		return lh.handleGetLoginPassState(line)
	case loginHandlerStateGetNewUser:
		return lh.handleGetNewUserState(line)
	case loginHandlerStateGetNewPass:
		return lh.handleGetNewPassState(line)
	default:
		retBytes := []byte("That's nice, dear.\n")
		return retBytes, lh, nil
	}
}

func (lh *loginHandler) handleLineInitState(terminalWidth, terminalHeight int) []byte {
	lh.currentMenu = &menu{
		options: []string{
			loginMenuItemLogin,
			loginMenuItemCreateAccount,
			loginMenuItemDisconnect,
		},
	}
	lh.state = loginHandlerStateFirstMenu

	return lh.currentMenu.init(terminalWidth, terminalHeight)
}

func (lh *loginHandler) handleLineFirstMenuState(line []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	outBytes, selection := lh.currentMenu.handleRxLine(line, terminalWidth, terminalHeight)
	if selection == "" {
		return outBytes, lh, nil
	}

	switch selection {
	case loginMenuItemLogin:
		lh.state = loginHandlerStateGetLoginUser
		return []byte("Username: "), lh, nil
	case loginMenuItemCreateAccount:
		lh.state = loginHandlerStateGetNewUser
		return []byte("New username: "), lh, nil
	case loginMenuItemDisconnect:
		return []byte("Ok, bye!\n"), nil, nil
	default:
		return nil, lh, nil
	}
}

func (lh *loginHandler) handleGetLoginUserState(line []byte) ([]byte, handler, error) {
	lh.username = string(line)
	lh.state = loginHandlerStateGetLoginPass
	return []byte("Password: "), lh, nil
}

func (lh *loginHandler) handleGetLoginPassState(line []byte) ([]byte, handler, error) {
	accountID, authZDesc, err := lh.authService.DoLogin(lh.username, string(line))
	if err == auth.BadUserOrPassphraseError {
		lh.state = loginHandlerStateInit
		return []byte(err.Error() + ", hit <enter> to continue."), lh, nil
	} else if err != nil {
		return nil, nil, err
	}

	newHandler := &lobbyHandler{
		authService: lh.authService,
		world:       lh.world,
		session:     lh.session,
		accountID:   accountID,
		authZDesc:   authZDesc,
	}

	return []byte("Successful login!\n"), newHandler, nil
}

func (lh *loginHandler) handleGetNewUserState(line []byte) ([]byte, handler, error) {
	lh.username = string(line)
	lh.state = loginHandlerStateGetNewPass
	return []byte("New passphrase: "), lh, nil
}

func (lh *loginHandler) handleGetNewPassState(line []byte) ([]byte, handler, error) {
	lh.state = loginHandlerStateInit
	pass := string(line)
	err := lh.authService.CreateAccount(lh.username, pass)
	if err != nil {
		return []byte(err.Error() + " Hit <enter> to continue."), lh, nil
	}
	return []byte("Account created! Hit <enter> to continue."), lh, nil
}

func (lh *loginHandler) deinit() {
	return
}

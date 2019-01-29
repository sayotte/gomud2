package telnet

import (
	"fmt"
	"github.com/derekparker/trie"
	"github.com/mitchellh/go-wordwrap"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/auth"
	"github.com/sayotte/gomud2/commands"
	"github.com/sayotte/gomud2/core"
	"sort"
	"strings"
)

type gameHandlerCommandHandler func(line string, terminalWidth int) ([]byte, error)

type gameHandler struct {
	authZDesc *auth.AuthZDescriptor
	session   *session

	actor *core.Actor
	world *core.World

	cmdTrie *trie.Trie
}

func (gh *gameHandler) init(terminalWidth, terminalHeight int) []byte {
	gh.cmdTrie = trie.New()
	gh.cmdTrie.Add("commands", gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandCommands(terminalWidth)
	}))
	gh.cmdTrie.Add("look", gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandLook(terminalWidth)
	}))
	gh.cmdTrie.Add("take", gh.getTakeHandler())
	gh.cmdTrie.Add("inventory", gh.getInventoryHandler())

	gh.cmdTrie.Add(core.ExitDirectionNorth, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.ExitDirectionNorth)
	}))
	gh.cmdTrie.Add(core.ExitDirectionSouth, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.ExitDirectionSouth)
	}))
	gh.cmdTrie.Add(core.ExitDirectionEast, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.ExitDirectionEast)
	}))
	gh.cmdTrie.Add(core.ExitDirectionWest, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.ExitDirectionWest)
	}))

	return lookAtLocation(core.ActorList{gh.actor}, terminalWidth, gh.actor.Location())
}

func (gh *gameHandler) handleEvent(e core.Event, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	switch e.Type() {
	case core.EventTypeActorMove:
		typedE := e.(*core.ActorMoveEvent)
		out, err := gh.handleEventActorMove(terminalWidth, typedE)
		return out, gh, err
	case core.EventTypeObjectMove:
		typedE := e.(*core.ObjectMoveEvent)
		out, err := gh.handleEventObjectMove(terminalWidth, typedE)
		return out, gh, err
	case core.EventTypeObjectRemoveFromZone:
		typedE := e.(core.ObjectRemoveFromZoneEvent)
		out, err := gh.handleEventObjectRemoved(terminalWidth, typedE)
		return out, gh, err
	default:
		return []byte(fmt.Sprintf("session: observed event of type %T\n", e)), gh, nil
	}
}

func (gh *gameHandler) handleRxLine(lineB []byte, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	line := string(lineB)
	if line == "" {
		return nil, gh, nil
	}

	firstTerm := strings.ToLower(strings.Split(line, " ")[0])
	terms := gh.cmdTrie.PrefixSearch(firstTerm)
	if len(terms) == 0 {
		return []byte(fmt.Sprintf("Unrecognized command %q, try \"commands\".\n", firstTerm)), gh, nil
	}
	sort.Strings(terms)
	node, _ := gh.cmdTrie.Find(terms[0])
	cmdHandler := node.Meta().(gameHandlerCommandHandler)

	restOfLine := strings.TrimLeft(strings.TrimPrefix(line, firstTerm), " ")

	outBytes, err := cmdHandler(restOfLine, terminalWidth)
	return outBytes, gh, err
}

func (gh *gameHandler) deinit() {
	gh.actor.RemoveObserver(gh.session)
}

func (gh *gameHandler) handleCommandLook(terminalWidth int) ([]byte, error) {
	return lookAtLocation(core.ActorList{gh.actor}, terminalWidth, gh.actor.Location()), nil
}

func (gh *gameHandler) handleCommandCommands(terminalWidth int) ([]byte, error) {
	return summarizeCommands(gh.cmdTrie, terminalWidth), nil
}

func (gh *gameHandler) handleCommandMoveGeneric(terminalWidth int, direction string) ([]byte, error) {
	newActor, err := commands.MoveActor(gh.actor, direction, gh.session)
	if err != nil {
		if commands.IsFatalError(err) {
			return []byte("ERROR!\n"), err
		}
		return []byte(err.Error() + "\n"), nil
	}
	gh.actor = newActor
	return nil, nil

	// FIXME zone must emit event when we do an AddToZone, and we must handle it
	// FIXME with a look, to omit the need for this look here
	//return gh.lookAtLocation(terminalWidth, gh.actor.Location()), nil
}

func (gh *gameHandler) handleEventActorMove(terminalWidth int, e *core.ActorMoveEvent) ([]byte, error) {
	fromID, toID, actorID := e.FromToActorIDs()

	from := gh.actor.Zone().LocationByID(fromID)
	to := gh.actor.Zone().LocationByID(toID)
	actor := gh.actor.Zone().ActorByID(actorID)
	actorName := "Someone"
	if actor != nil {
		actorName = actor.Name()
	}

	if actorID == gh.actor.ID() {
		// auto-look upon arriving at a new destination
		return lookAtLocation(core.ActorList{gh.actor}, terminalWidth, to), nil
	}

	if fromID == gh.actor.Location().ID() {
		// this is a departure
		outExit := exitRelativeToLocation(from, to)
		if outExit == nil {
			return []byte(fmt.Sprintf("%s departs to... somewhere.\n", actorName)), nil
		}
		return []byte(fmt.Sprintf("%s departs to the %s.\n", actorName, outExit.Direction())), nil

	} else if toID == gh.actor.Location().ID() {
		// this is an arrival
		outExit := exitRelativeToLocation(to, from)
		if outExit == nil {
			return []byte(fmt.Sprintf("%s arrives from... somewhere.\n", actorName)), nil
		}
		return []byte(fmt.Sprintf("%s arrives from the %s.\n", actorName, outExit.Direction())), nil
	} else {
		// the only way we can be getting this event is if we're subscribed to watching
		// someone else's actions
		return []byte(fmt.Sprintf("%s moves to %s.\n", actorName, to.ShortDescription())), nil
	}
}

func (gh *gameHandler) handleEventObjectMove(terminalWidth int, e *core.ObjectMoveEvent) ([]byte, error) {
	var out string

	zone := gh.actor.Zone()

	resolveActor := func(id uuid.UUID, defaultVal string) string {
		ret := defaultVal
		actor := zone.ActorByID(id)
		if actor != nil {
			ret = actor.Name()
		}
		return ret
	}
	resolveObj := func(id uuid.UUID) string {
		ret := "something"
		obj := zone.ObjectByID(id)
		if obj != nil {
			ret = obj.Name()
		}
		return ret
	}

	who := resolveActor(e.ActorID, "Someone")
	what := resolveObj(e.ObjectID)

	// These are the valid cases, which should be guaranteed by the command handler.
	// We simply ignore other cases-- they might be interesting, but this is a pretty annoying function
	// as it is.
	//| who == me? | fromActor | toActor | fromObject | toObject | fromLocation | toLocation | Description                                  |
	//|------------+-----------+---------+------------+----------+--------------+------------+----------------------------------------------|
	//| true       | me        | other   |            |          |              |            | You give X to <who>.                         |
	//| true       | me        |         |            | Y        |              |            | You put X in Y.                              |
	//| true       |           | me      | Y          |          |              |            | You take X from Y.                           |
	//| true       | me        |         |            |          |              | Y          | You drop X on the ground.                    |
	//| true       |           | me      |            |          | Y            |            | You pick up X from the ground.               |
	//| false      | other     | me      |            |          |              |            | <who> gives you X.                           |
	//| false      | other     |         |            | Y        |              |            | <who> puts X in Y.                           |
	//| false      |           | other   | Y          |          |              |            | <who> takes X from Y.                        |
	//| false      | other     |         |            |          |              | Y          | <who> drops X on the ground.                 |
	//| false      |           | other   |            |          | Y            |            | <who> picks up X from the ground.            |

	if uuid.Equal(e.ActorID, gh.actor.ID()) {
		switch {
		case !uuid.Equal(e.FromActorContainerID, uuid.Nil) && !uuid.Equal(e.ToActorContainerID, uuid.Nil):
			// me -> actor
			toWhom := resolveActor(e.ToActorContainerID, "someone")
			out = fmt.Sprintf("You give %s to %s.\n", what, toWhom)
		case !uuid.Equal(e.ToObjectContainerID, uuid.Nil):
			// me -> container
			intoWhat := resolveObj(e.ToObjectContainerID)
			out = fmt.Sprintf("You put %s into %s.\n", what, intoWhat)
		case !uuid.Equal(e.FromObjectContainerID, uuid.Nil):
			// container -> me
			fromWhat := resolveObj(e.FromObjectContainerID)
			out = fmt.Sprintf("You take %s from %s.\n", what, fromWhat)
		case !uuid.Equal(e.ToLocationContainerID, uuid.Nil):
			// me -> ground
			out = fmt.Sprintf("You drop %s on the ground.\n", what)
		case !uuid.Equal(e.FromLocationContainerID, uuid.Nil):
			// ground -> me
			out = fmt.Sprintf("You pick up %s from the ground.\n", what)
		}
	} else {
		switch {
		case !uuid.Equal(e.FromActorContainerID, uuid.Nil) && !uuid.Equal(e.ToActorContainerID, uuid.Nil):
			// actor -> me
			out = fmt.Sprintf("%s gives you %s.\n", who, what)
		case !uuid.Equal(e.ToObjectContainerID, uuid.Nil):
			// actor -> container
			intoWhat := resolveObj(e.ToObjectContainerID)
			out = fmt.Sprintf("%s puts %s into %s.\n", who, what, intoWhat)
		case !uuid.Equal(e.FromActorContainerID, uuid.Nil):
			// container -> actor
			fromWhat := resolveObj(e.FromObjectContainerID)
			out = fmt.Sprintf("%s takes %s from %s.\n", who, what, fromWhat)
		case !uuid.Equal(e.ToLocationContainerID, uuid.Nil):
			// actor -> ground
			out = fmt.Sprintf("%s drops %s on the ground.\n", who, what)
		case !uuid.Equal(e.FromLocationContainerID, uuid.Nil):
			// ground -> actor
			out = fmt.Sprintf("%s picks up %s from the ground.\n", who, what)
		}
	}

	return []byte(out), nil
}

func (gh *gameHandler) handleEventObjectRemoved(terminalWidth int, e core.ObjectRemoveFromZoneEvent) ([]byte, error) {
	return []byte(fmt.Sprintf("%s finally crumbles into dust.\n", e.Name)), nil
}

func (gh *gameHandler) getTakeHandler() gameHandlerCommandHandler {
	return func(line string, terminalWidth int) ([]byte, error) {
		params := strings.Split(line, " ")
		if len(params) == 0 {
			return []byte("Usage: take <object keyword>\n"), nil
		}

		targetKeyword := strings.ToLower(params[0])
		var targetObj *core.Object
		for _, obj := range gh.actor.Location().Objects() {
			if strings.HasPrefix(obj.Name(), targetKeyword) {
				targetObj = obj
				break
			}
		}
		if targetObj == nil {
			return []byte(fmt.Sprintf("Take what again? I can't find a %q.\n", targetKeyword)), nil
		}

		if len(gh.actor.Objects()) >= gh.actor.Capacity() {
			return []byte("You have no room for that in your inventory!\n"), nil
		}

		err := targetObj.Move(gh.actor.Location(), gh.actor, gh.actor)
		if err != nil {
			return []byte("Whoops...\n"), fmt.Errorf("Object.Move(Location, Actor): %s", err)
		}

		return nil, nil
	}
}

func (gh *gameHandler) getInventoryHandler() gameHandlerCommandHandler {
	return func(line string, terminalWidth int) ([]byte, error) {
		var objNames []string
		for _, obj := range gh.actor.Objects() {
			objNames = append(objNames, obj.Name())
		}

		return []byte(fmt.Sprintf("Inventory contents:\n%s\n\n", strings.Join(objNames, "\n"))), nil
	}
}

var locationExitDisplayOrder = []string{
	core.ExitDirectionNorth,
	core.ExitDirectionSouth,
	core.ExitDirectionEast,
	core.ExitDirectionWest,
}

func exitRelativeToLocation(baseLoc, otherLoc *core.Location) *core.Exit {
	for _, exit := range baseLoc.OutExits() {
		if exit.Destination() == otherLoc {
			return exit
		}
	}
	return nil
}

func lookAtLocation(ignoreActors core.ActorList, terminalWidth int, loc *core.Location) []byte {
	// short location description
	// long location description
	//
	// list of objects (one line)
	//
	// list of actors (one line each)
	//
	// list of exits
	lookFmt := "%s\n%s\n%s%s%s"

	var objClause string
	objects := loc.Objects()
	if len(objects) > 0 {
		objClause = "\nLaying on the ground, you see:\n"
		for _, obj := range loc.Objects() {
			objClause += fmt.Sprintf("%s\n", obj.Name())
		}
	}

	var actClause string
	actors := loc.Actors()
	if len(actors) > 0 {
		actClause = "\n"
		for _, actor := range actors {
			if _, err := ignoreActors.IndexOf(actor); err == nil {
				continue
			}
			actClause += actor.Name() + " is here.\n"
		}
	}

	var exitClause string
	exits := loc.OutExits()
	if len(exits) > 0 {
		exitClause = "\nObvious exits:\n"
		exitMap := make(map[string]*core.Exit)
		for _, exit := range exits {
			exitMap[exit.Direction()] = exit
		}
		for _, direction := range locationExitDisplayOrder {
			exit, found := exitMap[direction]
			if !found {
				continue
			}
			exitClause += fmt.Sprintf("%s\t- %s\n", direction, exit.Description())
		}
	}

	lookOutput := fmt.Sprintf(
		lookFmt,
		loc.ShortDescription(),
		wordwrap.WrapString(loc.Description(), uint(terminalWidth)),
		objClause,
		actClause,
		exitClause,
	)

	return []byte(lookOutput)
}

func summarizeCommands(cmdTrie *trie.Trie, terminalWidth int) []byte {
	allCmds := cmdTrie.Keys()
	sort.Strings(allCmds)

	// find the colWidth command, we'll format based on its width
	var colWidth int
	for _, cmd := range allCmds {
		if len(cmd) > colWidth {
			colWidth = len(cmd)
		}
	}
	colWidth += 2
	numCols := terminalWidth / colWidth

	var colNum int
	var output string
	for _, cmd := range allCmds {
		if colNum >= numCols {
			output += "\n"
			colNum = 0
		}
		padWidth := colWidth - 2 - len(cmd)
		output += cmd + strings.Repeat(" ", padWidth) + "  "
		colNum++
	}
	output += "\n"

	return []byte(output)
}

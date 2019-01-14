package telnet

import (
	"fmt"
	"github.com/derekparker/trie"
	"github.com/mitchellh/go-wordwrap"
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

	gh.cmdTrie.Add(core.EdgeDirectionNorth, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.EdgeDirectionNorth)
	}))
	gh.cmdTrie.Add(core.EdgeDirectionSouth, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.EdgeDirectionSouth)
	}))
	gh.cmdTrie.Add(core.EdgeDirectionEast, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.EdgeDirectionEast)
	}))
	gh.cmdTrie.Add(core.EdgeDirectionWest, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.EdgeDirectionWest)
	}))

	return gh.lookAtLocation(terminalWidth, gh.actor.Location())
}

func (gh *gameHandler) handleEvent(e core.Event, terminalWidth, terminalHeight int) ([]byte, handler, error) {
	switch e.Type() {
	case core.EventTypeActorMove:
		typedE := e.(*core.ActorMoveEvent)
		out, err := gh.handleEventActorMove(terminalWidth, typedE)
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
	node, _ := gh.cmdTrie.Find(terms[0])
	cmdHandler := node.Meta().(gameHandlerCommandHandler)

	restOfLine := strings.TrimPrefix(strings.TrimPrefix(line, firstTerm), "")

	outBytes, err := cmdHandler(restOfLine, terminalWidth)
	return outBytes, gh, err
}

func (gh *gameHandler) deinit() {
	gh.actor.RemoveObserver(gh.session)
}

func (gh *gameHandler) handleCommandLook(terminalWidth int) ([]byte, error) {
	return gh.lookAtLocation(terminalWidth, gh.actor.Location()), nil
}

func (gh *gameHandler) lookAtLocation(terminalWidth int, loc *core.Location) []byte {
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
			objClause += fmt.Sprintf("%s\n", obj.Name)
		}
	}

	var actClause string
	actors := loc.Actors()
	if len(actors) > 0 {
		actClause = "\n"
		for _, actor := range actors {
			if actor == gh.actor {
				continue
			}
			actClause += actor.Name() + " is here.\n"
		}
	}

	var exitClause string
	edges := loc.OutEdges
	if len(edges) > 0 {
		exitClause = "\nObvious exits:\n"
		exitMap := make(map[string]*core.LocationEdge)
		for _, edge := range edges {
			exitMap[edge.Direction] = edge
		}
		for _, direction := range locationExitDisplayOrder {
			edge, found := exitMap[direction]
			if !found {
				continue
			}
			exitClause += fmt.Sprintf("%s\t- %s\n", direction, edge.Description)
		}
	}

	lookOutput := fmt.Sprintf(
		lookFmt,
		loc.ShortDescription,
		wordwrap.WrapString(loc.Description, uint(terminalWidth)),
		objClause,
		actClause,
		exitClause,
	)

	return []byte(lookOutput)
}

func (gh *gameHandler) handleCommandCommands(terminalWidth int) ([]byte, error) {
	allCmds := gh.cmdTrie.Keys()
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

	return []byte(output), nil
}

func (gh *gameHandler) handleCommandMoveGeneric(terminalWidth int, direction string) ([]byte, error) {
	newActor, err := commands.MoveActor(gh.actor, direction, gh.session)
	if err != nil {
		if commands.IsFatalError(err) {
			return []byte("ERROR!\n"), err
		}
		return []byte(err.Error()), nil
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
		return gh.lookAtLocation(terminalWidth, to), nil
	}

	if fromID == gh.actor.Location().Id {
		// this is a departure
		outEdge := edgeRelativeToLocation(from, to)
		if outEdge == nil {
			return []byte(fmt.Sprintf("%s departs to... somewhere.\n", actorName)), nil
		}
		return []byte(fmt.Sprintf("%s departs to the %s.\n", actorName, outEdge.Direction)), nil

	} else if toID == gh.actor.Location().Id {
		// this is an arrival
		outEdge := edgeRelativeToLocation(to, from)
		if outEdge == nil {
			return []byte(fmt.Sprintf("%s arrives from... somewhere.\n", actorName)), nil
		}
		return []byte(fmt.Sprintf("%s arrives from the %s.\n", actorName, outEdge.Direction)), nil
	} else {
		// the only way we can be getting this event is if we're subscribed to watching
		// someone else's actions
		return []byte(fmt.Sprintf("%s moves to %s.\n", actorName, to.ShortDescription)), nil
	}
}

var locationExitDisplayOrder = []string{
	core.EdgeDirectionNorth,
	core.EdgeDirectionSouth,
	core.EdgeDirectionEast,
	core.EdgeDirectionWest,
}

func edgeRelativeToLocation(baseLoc, otherLoc *core.Location) *core.LocationEdge {
	for _, edge := range baseLoc.OutEdges {
		if edge.Destination == otherLoc {
			return edge
		}
	}
	return nil
}

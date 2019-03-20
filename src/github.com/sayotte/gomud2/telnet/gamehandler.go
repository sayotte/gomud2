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

	targetID uuid.UUID
}

func (gh *gameHandler) init(terminalWidth, terminalHeight int) []byte {
	gh.cmdTrie = trie.New()
	gh.cmdTrie.Add("commands", gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandCommands(terminalWidth)
	}))
	gh.cmdTrie.Add("drop", gh.getDropHandler())
	gh.cmdTrie.Add("inventory", gh.getInventoryHandler())
	gh.cmdTrie.Add("look", gh.getLookHandler())
	gh.cmdTrie.Add("put", gh.getPutHandler())
	gh.cmdTrie.Add("take", gh.getTakeHandler())
	gh.cmdTrie.Add("target", gh.getTargetHandler())
	gh.cmdTrie.Add("slash", gh.getSlashHandler())
	gh.cmdTrie.Add("kill", gh.getKillHandler())
	gh.cmdTrie.Add("wear", gh.getWearHandler())
	gh.cmdTrie.Add("remove", gh.getRemoveHandler())

	gh.cmdTrie.Add(core.ExitDirectionNorth, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.ExitDirectionNorth)
	}))
	gh.cmdTrie.Add("n", gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.ExitDirectionNorth)
	}))
	gh.cmdTrie.Add(core.ExitDirectionSouth, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.ExitDirectionSouth)
	}))
	gh.cmdTrie.Add("s", gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.ExitDirectionSouth)
	}))
	gh.cmdTrie.Add(core.ExitDirectionEast, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.ExitDirectionEast)
	}))
	gh.cmdTrie.Add("e", gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.ExitDirectionEast)
	}))
	gh.cmdTrie.Add(core.ExitDirectionWest, gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
		return gh.handleCommandMoveGeneric(terminalWidth, core.ExitDirectionWest)
	}))
	gh.cmdTrie.Add("w", gameHandlerCommandHandler(func(line string, terminalWidth int) ([]byte, error) {
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
	case core.EventTypeActorRemoveFromZone:
		// print nothing, this is not useful information for the Telnet client
		return nil, gh, nil
	case core.EventTypeActorDeath:
		typedE := e.(*core.ActorDeathEvent)
		out, err := gh.handleEventActorDeath(terminalWidth, typedE)
		return out, gh, err
	case core.EventTypeObjectMove:
		typedE := e.(*core.ObjectMoveEvent)
		out, err := gh.handleEventObjectMove(terminalWidth, typedE)
		return out, gh, err
	case core.EventTypeObjectMoveSubcontainer:
		typedE := e.(*core.ObjectMoveSubcontainerEvent)
		out, err := gh.handleEventObjectMoveSubcontainer(terminalWidth, typedE)
		return out, gh, err
	case core.EventTypeObjectRemoveFromZone:
		typedE := e.(*core.ObjectRemoveFromZoneEvent)
		out, err := gh.handleEventObjectRemoved(terminalWidth, typedE)
		return out, gh, err
	case core.EventTypeCombatMeleeDamage:
		typedE := e.(*core.CombatMeleeDamageEvent)
		out, err := gh.handleEventCombatMeleeDamage(terminalWidth, typedE)
		return out, gh, err
	case core.EventTypeCombatDodge:
		typedE := e.(*core.CombatDodgeEvent)
		out, err := gh.handleEventCombatDodge(terminalWidth, typedE)
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

func (gh *gameHandler) getLookHandler() gameHandlerCommandHandler {
	return func(line string, terminalWidth int) ([]byte, error) {
		params := strings.Split(line, " ")
		// If no args, just look at the location
		if len(params) == 0 || params[0] == "" {
			return lookAtLocation(core.ActorList{gh.actor}, terminalWidth, gh.actor.Location()), nil
		}

		targetKW := strings.ToLower(params[0])

		// If we're asked to look in a particular direction, look at the
		// Location in that direction (if there's even an Exit).
		var dirLook bool
		for _, dir := range orderedDirections {
			if targetKW == dir {
				dirLook = true
				break
			}
		}
		if dirLook {
			var exit *core.Exit
			for _, maybeExit := range gh.actor.Location().OutExits() {
				if maybeExit.Direction() == targetKW {
					exit = maybeExit
					break
				}
			}
			if exit == nil {
				return []byte("No exit in that direction!\n"), nil
			}
			var targetLoc *core.Location
			if exit.Destination() != nil {
				targetLoc = exit.Destination()
			} else {
				targetZone := gh.world.ZoneByID(exit.OtherZoneID())
				if targetZone != nil {
					targetLoc = targetZone.LocationByID(exit.OtherZoneLocID())
				}
			}
			if targetLoc == nil {
				return []byte("Weird, there's an exit that way, but it goes nowhere..."), nil
			}
			return lookAtLocation(core.ActorList{gh.actor}, terminalWidth, targetLoc), nil
		}

		// If we're asked to look at the special word "self", look at our own
		// Actor.
		if targetKW == "self" {
			return lookAtActor(terminalWidth, gh.actor), nil
		}

		// See if we've been asked to look at another Actor, by name/prefix.
		targetActor := nameActorMatch(targetKW, gh.actor.Location().Actors())
		if targetActor != nil {
			return lookAtActor(terminalWidth, targetActor), nil
		}

		// Otherwise, look at a particular object
		// Start by looking for a kw match in the inventory
		var targetObj *core.Object
		targetObj = keywordObjectMatch(targetKW, gh.actor.Objects())
		if targetObj == nil {
			// Failing that, look for a kw match on the ground
			targetObj = keywordObjectMatch(targetKW, gh.actor.Location().Objects())
			if targetObj == nil {
				return []byte(fmt.Sprintf("Look at what, exactly? I can't find a %q.\n", targetKW)), nil
			}
		}

		return lookAtObject(terminalWidth, targetObj), nil
	}
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

func (gh *gameHandler) handleEventActorDeath(terminalWidth int, e *core.ActorDeathEvent) ([]byte, error) {
	if uuid.Equal(e.ActorID, gh.actor.ID()) {
		return []byte("You died!\n"), nil
	}
	return []byte(fmt.Sprintf("%s died!\n", e.ActorName)), nil
}

func (gh *gameHandler) handleEventObjectMove(terminalWidth int, e *core.ObjectMoveEvent) ([]byte, error) {
	var out string

	zone := gh.actor.Zone()

	who := resolveActorNameByID(e.ActorID, "Someone", zone)
	what := resolveObjNameByID(e.ObjectID, zone)

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
			toWhom := resolveActorNameByID(e.ToActorContainerID, "someone", zone)
			out = fmt.Sprintf("You give %s to %s.\n", what, toWhom)
		case !uuid.Equal(e.ToObjectContainerID, uuid.Nil):
			// me -> container
			intoWhat := resolveObjNameByID(e.ToObjectContainerID, zone)
			out = fmt.Sprintf("You put %s into %s.\n", what, intoWhat)
		case !uuid.Equal(e.FromObjectContainerID, uuid.Nil):
			// container -> me
			fromWhat := resolveObjNameByID(e.FromObjectContainerID, zone)
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
			intoWhat := resolveObjNameByID(e.ToObjectContainerID, zone)
			out = fmt.Sprintf("%s puts %s into %s.\n", who, what, intoWhat)
		case !uuid.Equal(e.FromObjectContainerID, uuid.Nil):
			// container -> actor
			fromWhat := resolveObjNameByID(e.FromObjectContainerID, zone)
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

func (gh *gameHandler) handleEventObjectRemoved(terminalWidth int, e *core.ObjectRemoveFromZoneEvent) ([]byte, error) {
	return []byte(fmt.Sprintf("%s finally crumbles into dust.\n", e.Name)), nil
}

func (gh *gameHandler) handleEventObjectMoveSubcontainer(terminalWidth int, e *core.ObjectMoveSubcontainerEvent) ([]byte, error) {
	var out string

	zone := gh.actor.Zone()
	what := resolveObjNameByID(e.ObjectID, zone)

	// cases:
	// (self)  hands->other: you wear what on your where
	// (self)  other->hands: you remove what
	// (other) hands->other: who wears what on their where
	// (other) other->hands: who removes what
	if uuid.Equal(e.ActorID, gh.actor.ID()) {
		switch {
		case e.FromSubcontainer == core.InventoryContainerHands && e.ToSubcontainer != core.InventoryContainerHands:
			out = fmt.Sprintf("You wear %s on your %s.\n", what, e.ToSubcontainer)
		case e.FromSubcontainer != core.InventoryContainerHands && e.ToSubcontainer == core.InventoryContainerHands:
			out = fmt.Sprintf("You remove %s from your %s.\n", what, e.FromSubcontainer)
		default:
			out = fmt.Sprintf("You move %s from %s to %s.\n", what, e.FromSubcontainer, e.ToSubcontainer)
		}

	} else {
		who := resolveActorNameByID(e.ActorID, "Someone", zone)
		switch {
		case e.FromSubcontainer == core.InventoryContainerHands && e.ToSubcontainer != core.InventoryContainerHands:
			out = fmt.Sprintf("%s wears %s on their %s.", who, what, e.ToSubcontainer)
		case e.FromSubcontainer != core.InventoryContainerHands && e.ToSubcontainer == core.InventoryContainerHands:
			out = fmt.Sprintf("%s removes %s from their %s.\n", who, what, e.FromSubcontainer)
		default:
			out = fmt.Sprintf("%s moves %s from %s to %s.\n", who, what, e.FromSubcontainer, e.ToSubcontainer)
		}
	}

	return []byte(wordwrap.WrapString(out, uint(terminalWidth))), nil

}

func (gh *gameHandler) handleEventCombatMeleeDamage(terminalWidth int, e *core.CombatMeleeDamageEvent) ([]byte, error) {
	var attackerName, targetName string
	if uuid.Equal(e.AttackerID, gh.actor.ID()) {
		attackerName = "Your"
	} else {
		attackerName = fmt.Sprintf("%s's", e.AttackerName)
	}

	if uuid.Equal(e.TargetID, gh.actor.ID()) {
		targetName = "you"
	} else {
		targetName = e.TargetName
	}

	out := fmt.Sprintf("%s %s wounds %s.\n", attackerName, e.DamageType, targetName)
	return []byte(wordwrap.WrapString(out, uint(terminalWidth))), nil
}

func (gh *gameHandler) handleEventCombatDodge(terminalWidth int, e *core.CombatDodgeEvent) ([]byte, error) {
	targetName := "Someone"
	if uuid.Equal(e.TargetID, gh.actor.ID()) {
		targetName = "You"
	} else {
		for _, actor := range gh.actor.Location().Actors() {
			if uuid.Equal(e.TargetID, actor.ID()) {
				targetName = actor.Name()
				break
			}
		}
	}

	attackerName := "someone's"
	if uuid.Equal(e.AttackerID, gh.actor.ID()) {
		attackerName = "your"
	} else {
		for _, actor := range gh.actor.Location().Actors() {
			if uuid.Equal(e.AttackerID, actor.ID()) {
				attackerName = fmt.Sprintf("%s's", actor.Name())
				break
			}
		}
	}

	out := fmt.Sprintf("%s dodges %s %s.\n", targetName, attackerName, e.DamageType)
	return []byte(wordwrap.WrapString(out, uint(terminalWidth))), nil
}

func (gh *gameHandler) getTakeHandler() gameHandlerCommandHandler {
	return func(line string, terminalWidth int) ([]byte, error) {
		params := strings.Split(line, " ")
		if len(params) == 0 {
			return []byte("Usage: take <object keyword>\n"), nil
		}

		// Decide where we're taking the object from
		var container core.Container
		if len(params) == 1 {
			container = gh.actor.Location() // default to the ground
		} else {
			contKeyword := strings.ToLower(params[1])

			// first check inventory
			contObj := keywordObjectMatch(contKeyword, gh.actor.Objects())
			if contObj != nil {
				container = contObj
				goto foundContainer
			}

			// if that fails, check containers on the ground
			contObj = keywordObjectMatch(contKeyword, gh.actor.Location().Objects())
			if contObj != nil {
				container = contObj
				goto foundContainer
			}

			return []byte(fmt.Sprintf("Take from where? I can't find a %q.\n", contKeyword)), nil
		}
	foundContainer:

		// Decide which object we're taking
		targetKeyword := strings.ToLower(params[0])
		var targetObj *core.Object
		targetObj = keywordObjectMatch(targetKeyword, container.Objects())
		if targetObj == nil {
			return []byte(fmt.Sprintf("Take what again? I can't find a %q.\n", targetKeyword)), nil
		}

		//if len(gh.actor.Objects()) >= gh.actor.Capacity() {
		//	return []byte("You have no room for that in your inventory!\n"), nil
		//}
		handCapacity, handMaxItems := gh.actor.Inventory().CapacityBySubcontainer(core.InventoryContainerHands)
		handObjs := gh.actor.Inventory().ObjectsBySubcontainer(core.InventoryContainerHands)
		if len(handObjs) >= handMaxItems {
			return []byte("You don't have enough hands to hold that.\n"), nil
		}
		var handBurden int
		for _, handObj := range handObjs {
			handBurden += handObj.InventorySlots()
		}
		if handBurden+targetObj.InventorySlots() > handCapacity {
			return []byte("You're carrying too much in your hands to carry another thing.\n"), nil
		}

		err := targetObj.Move(container, gh.actor, gh.actor, core.ContainerDefaultSubcontainer)
		if err != nil {
			return []byte("Whoops...\n"), fmt.Errorf("Object.Move(Container, Actor): %s", err)
		}

		return nil, nil
	}
}

func (gh *gameHandler) getDropHandler() gameHandlerCommandHandler {
	return func(line string, terminalWidth int) ([]byte, error) {
		params := strings.Split(line, " ")
		if len(params) == 0 {
			return []byte("Usage: drop <object keyword>\n"), nil
		}

		targetKeyword := strings.ToLower(params[0])
		targetObj := keywordObjectMatch(targetKeyword, gh.actor.Inventory().ObjectsBySubcontainer(core.InventoryContainerHands))
		if targetObj == nil {
			return []byte(fmt.Sprintf("Drop what again? I can't find a %q.\n", targetKeyword)), nil
		}

		err := targetObj.Move(gh.actor, gh.actor.Location(), gh.actor, core.ContainerDefaultSubcontainer)
		if err != nil {
			return []byte("Whoops...\n"), fmt.Errorf("Object.Move(Actor, Location): %s", err)
		}

		return nil, nil
	}
}

func (gh *gameHandler) getPutHandler() gameHandlerCommandHandler {
	return func(line string, terminalWidth int) ([]byte, error) {
		params := strings.Split(line, " ")
		if len(params) <= 1 {
			return []byte("Usage: put <object keyword> <container keyword>\n"), nil
		}

		// Decide which object we're putting
		targetKeyword := strings.ToLower(params[0])
		targetObj := keywordObjectMatch(targetKeyword, gh.actor.Inventory().ObjectsBySubcontainer(core.InventoryContainerHands))
		if targetObj == nil {
			return []byte(fmt.Sprintf("Put what, exactly? There's no %q in your inventory.\n", targetKeyword)), nil
		}

		// Decide where we're putting it
		var container core.Container
		contKeyword := strings.ToLower(params[1])
		// first check inventory
		// note I'm working around a weirdness with nil-checking interface variables, see: https://gist.github.com/sayotte/450e5105f5004487646f84b3dc48e910
		contObj := keywordObjectMatch(contKeyword, gh.actor.Objects())
		if contObj != nil {
			container = contObj
			goto foundContainer
		}
		// if that fails, check containers on the ground
		contObj = keywordObjectMatch(contKeyword, gh.actor.Location().Objects())
		if contObj != nil {
			container = contObj
			goto foundContainer
		}
		return []byte(fmt.Sprintf("Put it where, exactly? I can't find a %q container.\n", contKeyword)), nil
	foundContainer:

		if len(container.Objects()) >= container.Capacity() {
			return []byte("That container can't hold any more.\n"), nil
		}

		err := targetObj.Move(targetObj.Container(), container, gh.actor, core.ContainerDefaultSubcontainer)
		if err != nil {
			return []byte("Whoops...\n"), fmt.Errorf("Object.Move(Actor, Container): %s", err)
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

func (gh *gameHandler) getTargetHandler() gameHandlerCommandHandler {
	return func(line string, terminalWidth int) ([]byte, error) {
		params := strings.Split(line, " ")
		if len(params) < 1 {
			return []byte("Usage: target <target keyword>\n"), nil
		}

		// Decide which actor we're targeting
		targetName := strings.ToLower(params[0])
		targetActor := nameActorMatch(targetName, gh.actor.Location().Actors())
		if targetActor == nil {
			return []byte(fmt.Sprintf("Target who, exactly? There's no %q here.\n", targetName)), nil
		}

		gh.targetID = targetActor.ID()
		return nil, nil
	}
}

func (gh *gameHandler) getSlashHandler() gameHandlerCommandHandler {
	return func(line string, terminalWidth int) ([]byte, error) {
		var targetActor *core.Actor
		for _, a := range gh.actor.Location().Actors() {
			if uuid.Equal(a.ID(), gh.targetID) {
				targetActor = a
				break
			}
		}
		if targetActor == nil {
			return []byte("Target doesn't seem to be in this location...\n"), nil
		}

		err := gh.actor.Slash(targetActor)
		if err != nil {
			return []byte("Whoops..."), fmt.Errorf("Actor.Slash(): %s", err)
		}

		return nil, nil
	}
}

func (gh *gameHandler) getKillHandler() gameHandlerCommandHandler {
	return func(line string, terminalWidth int) ([]byte, error) {
		var targetActor *core.Actor
		for _, a := range gh.actor.Location().Actors() {
			if uuid.Equal(a.ID(), gh.targetID) {
				targetActor = a
				break
			}
		}
		if targetActor == nil {
			return []byte("Target doesn't seem to be in this location...\n"), nil
		}

		err := targetActor.Die()
		if err != nil {
			return []byte("Whoops..."), fmt.Errorf("Actor.Die(): %s", err)
		}

		return nil, nil
	}
}

func (gh *gameHandler) getWearHandler() gameHandlerCommandHandler {
	return func(line string, terminalWidth int) ([]byte, error) {
		params := strings.Split(line, " ")
		if len(params) <= 1 {
			return []byte("Usage: wear <object keyword> <inventory slot keyword>\n"), nil
		}

		// Decide which object we're wearing
		targetKeyword := strings.ToLower(params[0])
		targetObj := keywordObjectMatch(targetKeyword, gh.actor.Inventory().ObjectsBySubcontainer(core.InventoryContainerHands))
		if targetObj == nil {
			return []byte(fmt.Sprintf("Wear what, exactly? You're not holding a %q in your hands.\n", targetKeyword)), nil
		}

		// Make sure we're not attempting something that'll throw an error for
		// obvious reasons, or is otherwise silly.
		if params[0] == core.InventoryContainerHands {
			return []byte("Hands are for holding things, not wearing them.\n"), nil
		}
		maxSlots, maxItems := gh.actor.Inventory().CapacityBySubcontainer(params[1])
		currentObjects := gh.actor.Inventory().ObjectsBySubcontainer(params[1])
		if len(currentObjects) >= maxItems {
			return []byte("You can't wear another item there.\n"), nil
		}
		currentSlotsTaken := 0
		for _, obj := range currentObjects {
			currentSlotsTaken += obj.InventorySlots()
		}
		if currentSlotsTaken+targetObj.InventorySlots() > maxSlots {
			return []byte("That item won't fit there.\n"), nil
		}

		err := targetObj.MoveToSubcontainer(params[1], gh.actor)
		if err != nil {
			return []byte("Whoops..."), err
		}

		return nil, nil
	}
}

func (gh *gameHandler) getRemoveHandler() gameHandlerCommandHandler {
	return func(line string, terminalWidth int) ([]byte, error) {
		params := strings.Split(line, " ")
		if len(params) < 1 {
			return []byte("Usage: remove <object keyword>\n"), nil
		}

		// Decide which object we're removing
		targetKeyword := strings.ToLower(params[0])
		targetObj := keywordObjectMatch(targetKeyword, gh.actor.Inventory().Objects())
		if targetObj == nil {
			return []byte(fmt.Sprintf("Remove what, exactly? You don't have a %q.\n", targetKeyword)), nil
		}

		// Make sure we're not attempting something that'll throw an error for
		// obvious reasons, or is otherwise silly.
		if targetObj.Container().SubcontainerFor(targetObj) == core.InventoryContainerHands {
			return []byte("You're already holding that in your hands, you can't remove it.\n"), nil
		}
		currentHandItems := gh.actor.Inventory().ObjectsBySubcontainer(core.InventoryContainerHands)
		maxSlots, maxItems := gh.actor.Inventory().CapacityBySubcontainer(core.InventoryContainerHands)
		if len(currentHandItems) >= maxItems {
			return []byte("You have no room in your hands to hold another item, put something down first?\n"), nil
		}
		currentSlotsTaken := 0
		for _, obj := range currentHandItems {
			currentSlotsTaken += obj.InventorySlots()
		}
		if currentSlotsTaken+targetObj.InventorySlots() > maxSlots {
			return []byte("That's too big to hold in your hands right now, put something down first?\n"), nil
		}

		err := targetObj.MoveToSubcontainer(core.InventoryContainerHands, gh.actor)
		if err != nil {
			return []byte("Whoops..."), err
		}

		return nil, nil
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

func lookAtObject(terminalWidth int, obj *core.Object) []byte {
	// name (short description)
	// long description
	// (optional) list of contained objects
	lookFmt := "%s\n%s\n%s\n"

	var containedObjsClause string
	if obj.Capacity() > 0 {
		var objNames []string
		for _, o := range obj.Objects() {
			objNames = append(objNames, o.Name())
		}
		containedObjsClause = fmt.Sprintf("\nPeering inside, you see:\n%s\n", strings.Join(objNames, "\n"))
	}

	lookOutput := fmt.Sprintf(
		lookFmt,
		obj.Name(),
		wordwrap.WrapString(obj.Description(), uint(terminalWidth)),
		containedObjsClause,
	)

	return []byte(lookOutput)
}

func lookAtActor(terminalWidth int, actor *core.Actor) []byte {
	// You see <name>.
	// They are wearing:
	//   on their body: %s
	//   ...
	//   on their back: %s
	//   ...
	//   on their belt: %s
	//   ...
	//   in their hands: %s
	//   ...
	lookFmt := "You see %s.\n%s\n"

	var wornObjectsClause string
	onSubcontainerNames := []string{
		core.InventoryContainerBody,
		core.InventoryContainerBack,
		core.InventoryContainerBelt,
	}
	onSubFmt := "  on their %s: %s\n"
	for _, subName := range onSubcontainerNames {
		for _, obj := range actor.Inventory().ObjectsBySubcontainer(subName) {
			wornObjectsClause += fmt.Sprintf(onSubFmt, subName, obj.Name())
		}
	}
	for _, obj := range actor.Inventory().ObjectsBySubcontainer(core.InventoryContainerHands) {
		wornObjectsClause += fmt.Sprintf("  in their hands: %s\n", obj.Name())
	}
	if len(wornObjectsClause) > 0 {
		wornObjectsClause = "They are wearing:\n" + wornObjectsClause
	}

	out := fmt.Sprintf(lookFmt, actor.Name(), wornObjectsClause)
	return []byte(wordwrap.WrapString(out, uint(terminalWidth)))
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

func keywordObjectMatch(keyword string, candidateObjs []*core.Object) *core.Object {
	for _, obj := range candidateObjs {
		for _, kw := range obj.Keywords() {
			if strings.HasPrefix(kw, keyword) {
				return obj
			}
		}
	}
	return nil
}

func nameActorMatch(name string, candidateActors core.ActorList) *core.Actor {
	lowerName := strings.ToLower(name)
	for _, a := range candidateActors {
		if strings.HasPrefix(strings.ToLower(a.Name()), lowerName) {
			return a
		}
	}
	return nil
}

func resolveActorNameByID(id uuid.UUID, defaultVal string, zone *core.Zone) string {
	ret := defaultVal
	actor := zone.ActorByID(id)
	if actor != nil {
		ret = actor.Name()
	}
	return ret
}

func resolveObjNameByID(id uuid.UUID, zone *core.Zone) string {
	ret := "something"
	obj := zone.ObjectByID(id)
	if obj != nil {
		ret = obj.Name()
	}
	return ret
}

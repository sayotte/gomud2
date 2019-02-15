package core

import (
	"errors"
	"fmt"
)

const (
	// ascending powers of 4
	ObjectSizeTinySlots   = 1   // e.g. a marble
	ObjectSizeSmallSlots  = 4   // e.g. a dagger or belt-pouch
	ObjectSizeMediumSlots = 16  // e.g. a sword
	ObjectSizeLargeSlots  = 64  // e.g. a breastplate or polearm
	ObjectSizeHugeSlots   = 256 // e.g. a giant's two-handed club
)

const (
	InventoryContainerBack  = "back"
	InventoryContainerBelt  = "belt"
	InventoryContainerBody  = "body"
	InventoryContainerHands = "hands"
)

var DefaultHumanInventoryConstraints = ActorInventoryConstraints{
	BackSlots:    1 * ObjectSizeLargeSlots,
	BackMaxItems: 2,
	BeltSlots:    2 * ObjectSizeMediumSlots,
	BeltMaxItems: 4,
	BodySlots:    1 * ObjectSizeLargeSlots,
	BodyMaxItems: 1,
	HandSlots:    1 * ObjectSizeLargeSlots,
	HandMaxItems: 2,
}

type ActorInventoryConstraints struct {
	BackSlots    int
	BackMaxItems int
	BeltSlots    int
	BeltMaxItems int
	BodySlots    int
	BodyMaxItems int
	HandSlots    int
	HandMaxItems int
}

type ActorInventory struct {
	constraints ActorInventoryConstraints
	back        ObjectList
	belt        ObjectList
	body        ObjectList
	hands       ObjectList
}

func (ai *ActorInventory) Constraints() ActorInventoryConstraints {
	return ai.constraints
}

func (ai *ActorInventory) Capacity() int {
	return ai.constraints.BackSlots + ai.constraints.BeltSlots + ai.constraints.BodySlots + ai.constraints.HandSlots
}

func (ai *ActorInventory) ContainsObject(o *Object) bool {
	_, err := ai.back.IndexOf(o)
	if err == nil {
		return true
	}

	_, err = ai.belt.IndexOf(o)
	if err == nil {
		return true
	}

	_, err = ai.body.IndexOf(o)
	if err == nil {
		return true
	}

	_, err = ai.hands.IndexOf(o)
	if err == nil {
		return true
	}

	return false
}

func (ai *ActorInventory) SubcontainerFor(o *Object) string {
	_, err := ai.back.IndexOf(o)
	if err == nil {
		return InventoryContainerBack
	}
	_, err = ai.belt.IndexOf(o)
	if err == nil {
		return InventoryContainerBelt
	}
	_, err = ai.body.IndexOf(o)
	if err == nil {
		return InventoryContainerBody
	}
	_, err = ai.hands.IndexOf(o)
	if err == nil {
		return InventoryContainerHands
	}
	return ""
}

func (ai *ActorInventory) Objects() ObjectList {
	listLen := len(ai.back) + len(ai.belt) + len(ai.body) + len(ai.hands)
	ol := make(ObjectList, listLen)
	idx := 0
	copy(ol[idx:], ai.back)
	idx += len(ai.back)
	copy(ol[idx:], ai.belt)
	idx += len(ai.belt)
	copy(ol[idx:], ai.body)
	idx += len(ai.body)
	copy(ol[idx:], ai.hands)

	return ol
}

func (ai *ActorInventory) ObjectsBySubcontainer(subname string) ObjectList {
	switch subname {
	case InventoryContainerBack:
		return ai.back.Copy()
	case InventoryContainerBelt:
		return ai.belt.Copy()
	case InventoryContainerBody:
		return ai.body.Copy()
	case InventoryContainerHands:
		return ai.hands.Copy()
	}
	return nil
}

func (ai *ActorInventory) addObject(o *Object, subContainer string) error {
	// objects always go into the hands first; they can be moved to other
	// inventory locations afterward

	if len(ai.hands) >= ai.constraints.HandMaxItems {
		return fmt.Errorf("hands are full")
	}

	var slotsTaken int
	for _, o := range ai.hands {
		slotsTaken += o.InventorySlots()
	}
	if slotsTaken+o.InventorySlots() > ai.constraints.HandSlots {
		return fmt.Errorf("object too large to hold")
	}

	ai.hands = append(ai.hands, o)
	return nil
}

func (ai *ActorInventory) removeObject(o *Object) {
	ai.back = ai.back.Remove(o)
	ai.belt = ai.belt.Remove(o)
	ai.body = ai.body.Remove(o)
	ai.hands = ai.hands.Remove(o)
}

func (ai *ActorInventory) checkMoveObjectToSubcontainer(o *Object, srcSub, dstSub string) error {
	var srcList, dstList ObjectList
	var dstSlots, dstMaxItems int

	switch srcSub {
	case InventoryContainerBack:
		srcList = ai.back
	case InventoryContainerBelt:
		srcList = ai.belt
	case InventoryContainerBody:
		srcList = ai.body
	case InventoryContainerHands:
		srcList = ai.hands
	default:
		return fmt.Errorf("unknown subcontainer %q", srcSub)
	}

	switch dstSub {
	case InventoryContainerBack:
		dstList = ai.back
		dstSlots = ai.constraints.BackSlots
		dstMaxItems = ai.constraints.BackMaxItems
	case InventoryContainerBelt:
		dstList = ai.belt
		dstSlots = ai.constraints.BeltSlots
		dstMaxItems = ai.constraints.BeltMaxItems
	case InventoryContainerBody:
		dstList = ai.body
		dstSlots = ai.constraints.BodySlots
		dstMaxItems = ai.constraints.BodyMaxItems
	case InventoryContainerHands:
		dstList = ai.hands
		dstSlots = ai.constraints.HandSlots
		dstMaxItems = ai.constraints.HandMaxItems
	default:
		return fmt.Errorf("unknown subcontainer %q", dstSub)
	}

	check := func(o *Object, srcList, dstList ObjectList, dstSlots, dstMaxItems int) error {
		_, err := srcList.IndexOf(o)
		if err != nil {
			return errors.New("Object not found in source subcontainer")
		}
		var dstSLotsTaken int
		for _, dstO := range dstList {
			dstSLotsTaken += dstO.inventorySlots
		}
		if dstSLotsTaken+o.inventorySlots > dstSlots {
			return errors.New("object can't fit in that subcontainer")
		}
		if len(dstList)+1 > dstMaxItems {
			return errors.New("subcontainer can't fit another item of any size")
		}

		return nil
	}

	return check(o, srcList, dstList, dstSlots, dstMaxItems)
}

func (ai *ActorInventory) moveObjectToSubcontainer(o *Object, srcSub, dstSub string) error {
	switch srcSub {
	case InventoryContainerBack:
		ai.back = ai.back.Remove(o)
	case InventoryContainerBelt:
		ai.belt = ai.belt.Remove(o)
	case InventoryContainerBody:
		ai.body = ai.body.Remove(o)
	case InventoryContainerHands:
		ai.hands = ai.hands.Remove(o)
	default:
		return fmt.Errorf("unknown subcontainer %q", srcSub)
	}

	switch dstSub {
	case InventoryContainerBack:
		ai.back = append(ai.back, o)
	case InventoryContainerBelt:
		ai.belt = append(ai.belt, o)
	case InventoryContainerBody:
		ai.body = append(ai.body, o)
	case InventoryContainerHands:
		ai.hands = append(ai.hands, o)
	default:
		return fmt.Errorf("unknown subcontainer %q", srcSub)
	}

	return nil
}

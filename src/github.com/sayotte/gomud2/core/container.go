package core

import "github.com/satori/go.uuid"

const ContainerDefaultSubcontainer = "any"

type Container interface {
	ID() uuid.UUID
	Capacity() int
	ContainsObject(o *Object) bool
	SubcontainerFor(o *Object) string
	Objects() ObjectList
	Observers() ObserverList
	Location() *Location
	addObject(o *Object, subcontainer string) error
	removeObject(o *Object)
	checkMoveObjectToSubcontainer(o *Object, oldSub, newSub string) error
	moveObjectToSubcontainer(o *Object, oldSub, newSub string) error
}

type objectContainerTuple struct {
	obj  *Object
	cont Container
}

func getObjectContainerTuplesRecursive(cont Container) []objectContainerTuple {
	var out []objectContainerTuple
	for _, obj := range cont.Objects() {
		out = append(out, objectContainerTuple{
			obj:  obj,
			cont: cont,
		})
		out = append(out, getObjectContainerTuplesRecursive(obj)...)
	}
	return out
}

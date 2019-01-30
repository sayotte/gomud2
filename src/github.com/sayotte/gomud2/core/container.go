package core

import "github.com/satori/go.uuid"

type Container interface {
	ID() uuid.UUID
	Capacity() int
	ContainsObject(o *Object) bool
	Objects() ObjectList
	Observers() ObserverList
	Location() *Location
	addObject(o *Object) error
	removeObject(o *Object)
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

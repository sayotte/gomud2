package core

import "github.com/satori/go.uuid"

type Container interface {
	ID() uuid.UUID
	Capacity() int
	ContainsObject(o *Object) bool
	Objects() ObjectList
	Observers() ObserverList
	addObject(o *Object) error
	removeObject(o *Object)
}

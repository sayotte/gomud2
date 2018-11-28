package uuid

import (
	"bytes"
	"github.com/satori/go.uuid"
)

func NewId() uuid.UUID {
	id, _ := uuid.NewV4()
	return id
}

type UUIDList []uuid.UUID

func (ul UUIDList) Len() int {
	return len(ul)
}

func (ul UUIDList) Swap(i, j int) {
	ul[i], ul[j] = ul[j], ul[i]
}

func (ul UUIDList) Less(i, j int) bool {
	return bytes.Compare(ul[i][:], ul[j][:]) == -1
}

func (ul UUIDList) IndexOf(id uuid.UUID) int {
	for i := 0; i < len(ul); i++ {
		if uuid.Equal(id, ul[i]) {
			return i
		}
	}
	return -1
}

func (ul UUIDList) Remove(id uuid.UUID) UUIDList {
	idx := ul.IndexOf(id)
	if idx == -1 {
		return ul
	}
	return append(ul[:idx], ul[idx+1:]...)
}

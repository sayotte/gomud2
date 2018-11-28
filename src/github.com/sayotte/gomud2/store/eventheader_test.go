package store

import (
	"reflect"
	"testing"

	"github.com/satori/go.uuid"
)

func TestEventHeader_MarshalBinary(t *testing.T) {
	id, _ := uuid.NewV4()
	eh := eventHeader{
		EventType:      43,
		Version:        9,
		AggregateId:    id,
		SequenceNumber: 199,
		Length:         457,
	}
	buf, _ := eh.MarshalBinary()
	var newEH eventHeader
	err := (&newEH).UnmarshalBinary(buf)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if !reflect.DeepEqual(eh, newEH) {
		t.Errorf("%v != %v", eh, newEH)
	}
}

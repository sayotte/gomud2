package store

import (
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
	myuuid "github.com/sayotte/gomud2/uuid"
	"testing"
)

func TestExitAddToZoneEvent_roundtrip(t *testing.T) {
	cmpDomainLeatze := func(left, right core.ExitAddToZoneEvent) (bool, string) {
		if left.Type() != right.Type() {
			return false, fmt.Sprintf("Type(): %d != %d", left.Type(), right.Type())
		}
		if left.Version() != right.Version() {
			return false, fmt.Sprintf("Version(): %d != %d", left.Version(), right.Version())
		}
		if !uuid.Equal(left.AggregateId(), right.AggregateId()) {
			return false, fmt.Sprintf("aggregateID: %q != %q", left.AggregateId(), right.AggregateId())
		}
		if left.SequenceNumber() != right.SequenceNumber() {
			return false, fmt.Sprintf("SequenceNumber(): %d != %q", left.SequenceNumber(), right.SequenceNumber())
		}
		if left.ShouldPersist() != right.ShouldPersist() {
			return false, fmt.Sprintf("ShouldPersist(): %t != %t", left.ShouldPersist(), right.ShouldPersist())
		}

		if !uuid.Equal(left.ExitID, right.ExitID) {
			return false, fmt.Sprintf("ExitID: %q != %q", left.ExitID, right.ExitID)
		}
		if left.Description != right.Description {
			return false, fmt.Sprintf("description: %q != %q", left.Description, right.Description)
		}
		if left.Direction != right.Direction {
			return false, fmt.Sprintf("Direction: %s != %s", left.Direction, right.Direction)
		}
		if !uuid.Equal(left.SourceLocationId, right.SourceLocationId) {
			return false, fmt.Sprintf("SourceLocationID: %q != %q", left.SourceLocationId, right.SourceLocationId)
		}
		if !uuid.Equal(left.DestZoneID, right.DestZoneID) {
			return false, fmt.Sprintf("DestZoneID: %q != %q", left.DestZoneID, right.DestZoneID)
		}
		if !uuid.Equal(left.DestLocationId, right.DestLocationId) {
			return false, fmt.Sprintf("DestLocationID: %q != %q", left.DestLocationId, right.DestLocationId)
		}

		return true, ""
	}

	inEvent := core.NewExitAddToZoneEvent(
		"desc",
		"direction",
		myuuid.NewId(),
		myuuid.NewId(),
		myuuid.NewId(),
		myuuid.NewId(),
		myuuid.NewId(),
	)
	inEvent.SetSequenceNumber(97)

	leatze := &exitAddToZoneEvent{}
	leatze.FromDomain(inEvent)
	outEvent := leatze.ToDomain()

	same, why := cmpDomainLeatze(inEvent, outEvent.(core.ExitAddToZoneEvent))
	if !same {
		t.Error(why)
	}
}

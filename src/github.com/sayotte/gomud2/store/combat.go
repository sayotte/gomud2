package store

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
)

type combatMeleeDamageEvent struct {
	header                            eventHeader
	DamageType                        string
	AttackerID                        uuid.UUID
	TargetID                          uuid.UUID
	PhysicalDmg, StaminaDmg, FocusDmg int
}

func (cmde combatMeleeDamageEvent) ToDomain() core.Event {
	e := core.NewCombatMeleeDamageEvent(
		cmde.DamageType,
		cmde.AttackerID,
		cmde.TargetID,
		cmde.header.AggregateId,
		cmde.PhysicalDmg,
		cmde.StaminaDmg,
		cmde.FocusDmg,
	)
	e.SetSequenceNumber(cmde.header.SequenceNumber)
	e.SetTimestamp(cmde.header.Timestamp)
	return e
}

func (cmde *combatMeleeDamageEvent) FromDomain(e core.Event) {
	from := e.(*core.CombatMeleeDamageEvent)
	*cmde = combatMeleeDamageEvent{
		header:      eventHeaderFromDomainEvent(from),
		DamageType:  from.DamageType,
		AttackerID:  from.AttackerID,
		TargetID:    from.TargetID,
		PhysicalDmg: from.PhysicalDmg,
		StaminaDmg:  from.StaminaDmg,
		FocusDmg:    from.StaminaDmg,
	}
}

func (cmde combatMeleeDamageEvent) Header() eventHeader {
	return cmde.header
}

func (cmde *combatMeleeDamageEvent) SetHeader(h eventHeader) {
	cmde.header = h
}

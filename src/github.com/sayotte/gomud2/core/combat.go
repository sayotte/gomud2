package core

import (
	"fmt"
	"github.com/satori/go.uuid"
	"math"
	"math/rand"
	"time"
)

// returns a random float64 between 0.0 and 1.0
func rollFloat64(r *rand.Rand) float64 {
	val := float64(r.Float32())
	// scale val to 0.0 - 1.0
	// we could use this func:
	//    scaleFloat64 := func(x, minX, maxX, targetMin, targetMax float64) float64 {
	//	    return (targetMax - targetMin) * ((x - minX) / (maxX - minX)) + targetMin
	//    }
	// but since we know our input/target ranges are always the same, we can simplify
	return (val + float64(math.MaxFloat32)) / (2 * float64(math.MaxFloat32))
}

const (
	combatDodgeBaseChance                      = 20.0
	combatDodgeTechniquesUsableAtSkillInterval = 20.0
)

const (
	CombatMeleeDamageTypeSlash = "slash"
	CombatMeleeDamageTypeStab  = "stab"
	CombatMeleeDamageTypeBash  = "bash"

	// reserved for NPCs
	CombatMeleeDamageTypeBite = "bite"
)

func newCombatMeleeCommand(attacker, target *Actor, dmgType string) *combatMeleeCommand {
	return &combatMeleeCommand{
		commandGeneric: commandGeneric{commandType: CommandTypeCombatMelee},
		attacker:       attacker,
		target:         target,
		damageType:     dmgType,
	}
}

type combatMeleeCommand struct {
	commandGeneric
	attacker, target *Actor
	damageType       string
}

func (cmc combatMeleeCommand) Do() ([]Event, error) {
	switch cmc.damageType {
	case CombatMeleeDamageTypeSlash:
		return cmc.doSlash()
	default:
		return nil, fmt.Errorf("don't know how to compute damage type %q", cmc.damageType)
	}
}

func (cmc combatMeleeCommand) doSlash() ([]Event, error) {
	if cmc.checkDodge(cmc.attacker.Skills().Slashing, cmc.target) {
		dodgeEvent := NewCombatDodgeEvent(CombatMeleeDamageTypeSlash, cmc.attacker.Name(), cmc.target.Name(), cmc.attacker.ID(), cmc.target.ID(), cmc.attacker.Zone().ID())
		return []Event{dodgeEvent}, nil
	}

	// find weapon in attacker's hands with highest slashing damage cap; use
	// that for damage range
	var weaponMinBaseDmg, weaponMaxBaseDmg float64
	for _, obj := range cmc.attacker.Inventory().ObjectsBySubcontainer(InventoryContainerHands) {
		attrs := obj.Attributes()
		if attrs.SlashingDamageMax > weaponMaxBaseDmg {
			weaponMinBaseDmg = attrs.SlashingDamageMin
			weaponMaxBaseDmg = attrs.SlashingDamageMax
		}
	}
	// calculate damage after bonuses etc.
	baseDmgRange := weaponMaxBaseDmg - weaponMinBaseDmg
	scaledBaseDmg := (rollFloat64(cmc.attacker.Zone().Rand()) * baseDmgRange) + weaponMinBaseDmg
	physBonus := (float64(cmc.attacker.Attributes().Physical) / 100) * scaledBaseDmg // max 0.50
	focBonus := (float64(cmc.attacker.Attributes().Focus) / 100) * scaledBaseDmg     // max 0.15
	totalDmg := scaledBaseDmg + physBonus + focBonus

	// distribute damage 3:1:1 over phys:stam:focus
	physDmg := int(math.Round(totalDmg * 0.60))
	stamDmg := int(math.Round(totalDmg * 0.20))
	focDmg := int(math.Round(totalDmg * 0.20))

	damageEvent := NewCombatMeleeDamageEvent(
		CombatMeleeDamageTypeSlash,
		cmc.attacker.ID(),
		cmc.target.ID(),
		cmc.attacker.Zone().ID(),
		cmc.attacker.Name(),
		cmc.target.Name(),
		physDmg,
		stamDmg,
		focDmg,
	)

	outEvents := []Event{damageEvent}

	switch {
	case cmc.target.attributes.Physical-damageEvent.PhysicalDmg <= 0:
		fallthrough
	case cmc.target.attributes.Stamina-damageEvent.StaminaDmg <= 0:
		fallthrough
	case cmc.target.attributes.Focus-damageEvent.FocusDmg <= 0:
		deathEvents := doActorDeath(cmc.target, cmc.target.Zone())
		outEvents = append(outEvents, deathEvents...)
	}

	return outEvents, nil
}

func (cmc combatMeleeCommand) checkDodge(attackSkill float64, defender *Actor) bool {
	dSkills := defender.Skills()
	defendSkill := dSkills.Dodging

	// scale our % chance to the difference between attacker/defender skills
	skillScale := defendSkill - attackSkill
	// clamp it to -50/+50, then add 50 so it's 0-100
	if skillScale < -50 {
		skillScale = -50
	} else if skillScale > 50 {
		skillScale = 50
	}
	skillScale += 50

	scaledChance := (skillScale / 100) * combatDodgeBaseChance
	stamBonus := float64(defender.Attributes().Stamina) / 100
	focBonus := float64(defender.Attributes().Focus) / 100
	chance := scaledChance + stamBonus + focBonus

	for i := 1; i <= dSkills.DodgingTechniques; i++ {
		// e.g. if we have 39.0 skill, we shouldn't be able to try a second technique
		if float64(i)*combatDodgeTechniquesUsableAtSkillInterval > dSkills.Dodging {
			break
		}
		roll := rollFloat64(defender.Zone().Rand()) * 100
		if roll <= chance {
			return true
		}
	}
	return false
}

func NewCombatMeleeDamageEvent(dmgTyp string, attackerID, targetID, zoneID uuid.UUID, attackerName, targetName string, physDmg, stamDmg, focDmg int) *CombatMeleeDamageEvent {
	return &CombatMeleeDamageEvent{
		eventGeneric: &eventGeneric{
			EventTypeNum:      EventTypeCombatMeleeDamage,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: true,
		},
		DamageType:   dmgTyp,
		AttackerID:   attackerID,
		TargetID:     targetID,
		AttackerName: attackerName,
		TargetName:   targetName,
		PhysicalDmg:  physDmg,
		StaminaDmg:   stamDmg,
		FocusDmg:     focDmg,
	}
}

type CombatMeleeDamageEvent struct {
	*eventGeneric
	DamageType                        string
	AttackerID                        uuid.UUID
	TargetID                          uuid.UUID
	AttackerName, TargetName          string
	PhysicalDmg, StaminaDmg, FocusDmg int
}

func NewCombatDodgeEvent(dmgType, attackerName, targetName string, attackerID, targetID, zoneID uuid.UUID) *CombatDodgeEvent {
	return &CombatDodgeEvent{
		eventGeneric: &eventGeneric{
			EventTypeNum:      EventTypeCombatDodge,
			TimeStamp:         time.Now(),
			VersionNum:        1,
			AggregateID:       zoneID,
			ShouldPersistBool: false,
		},
		DamageType:   dmgType,
		AttackerName: attackerName,
		TargetName:   targetName,
		AttackerID:   attackerID,
		TargetID:     targetID,
	}
}

type CombatDodgeEvent struct {
	*eventGeneric
	DamageType               string
	AttackerID               uuid.UUID
	TargetID                 uuid.UUID
	AttackerName, TargetName string
}

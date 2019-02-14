package core

type Skillset struct {
	// melee attack skills
	Slashing, SlashingCap float64
	Stabbing, StabbingCap float64
	Bashing, BashingCap   float64
	// NPC melee attack skills
	Biting, BitingCap float64

	// melee defense skills
	Dodging, DodgingCap                     float64
	DodgingTechniques, DodgingTechniquesCap int
	Deflecting, DeflectingCap               float64
	Blocking, BlockingCap                   float64

	// magic skills
	Sorcery, SorceryCap         float64
	Mysticism, MysticismCap     float64
	Inscription, InscriptionCap float64
}

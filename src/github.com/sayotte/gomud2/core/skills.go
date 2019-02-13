package core

type Skillset struct {
	// melee attack skills
	Slashing, SlashingCap int
	Stabbing, StabbingCap int
	Bashing, BashingCap   int
	// NPC melee attack skills
	Biting, BitingCap int

	// melee defense skills
	Dodging, DodgingCap       int
	Deflecting, DeflectingCap int
	Blocking, BlockingCap     int

	// magic skills
	Sorcery, SorceryCap         int
	Mysticism, MysticismCap     int
	Inscription, InscriptionCap int
}

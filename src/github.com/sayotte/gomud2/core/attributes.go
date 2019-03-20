package core

type AttributeSet struct {
	TotalBaseCap int
	// base attributes / caps
	Strength, StrengthCap int
	Fitness, FitnessCap   int
	Will, WillCap         int
	Faith, Faithcap       int
	// derived attributes
	Physical, Stamina, Focus, Zeal int
	// natural combat stats
	NaturalBiteMin, NaturalBiteMax   float64
	NaturalSlashMin, NaturalSlashMax float64
}

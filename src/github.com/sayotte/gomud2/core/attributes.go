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
	NaturalSlashMin, NaturalSlashMax float64
}

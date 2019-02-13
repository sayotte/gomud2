package spawnreap

import (
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
)

type SpawnSpecification struct {
	ActorProto         ActorPrototype
	MaxCount           int
	MaxSpawnAtOneTime  int
	SpawnChancePerTick float64
}

type ActorPrototype struct {
	Name       string
	BrainType  string
	Attributes core.AttributeSet
	Skills     core.Skillset
}

func (ap ActorPrototype) ToActor(loc *core.Location) *core.Actor {
	return core.NewActor(
		uuid.Nil,
		ap.Name,
		ap.BrainType,
		loc,
		loc.Zone(),
		ap.Attributes,
		ap.Skills,
	)
}

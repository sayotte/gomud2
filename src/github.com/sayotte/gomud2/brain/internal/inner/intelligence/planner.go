package intelligence

import (
	"fmt"
	"time"

	"github.com/satori/go.uuid"
)

const (
	planGoalDoNothing           = "do-nothing"
	planGoalMoveToEmptyLocation = "move-to-emptier-location"
	planGoalDefendSelf          = "defend-self"
)

type planner struct {
	actorID               uuid.UUID
	memory                *Memory
	goalSelector          UtilitySelector
	currentGoal           string
	currentGoalSelectedAt time.Time
	minTimeToRegoal       time.Duration
	currentPlanSelectedAt time.Time
	maxTimeToReplan       time.Duration
}

func (p *planner) generatePlan(oldPlan executionPlan) executionPlan {
	//p.memory.lock.RLock()
	//fmt.Println("\n\nDUMPING MEMORY")
	//memKeys := make([]string, 0, len(p.memory.localStore))
	//for k := range p.memory.localStore {
	//	memKeys = append(memKeys, k)
	//}
	//sort.Strings(memKeys)
	//for _, k := range memKeys {
	//	v := p.memory.localStore[k]
	//	fmt.Printf("==%s==\n", k)
	//	p, _ := json.MarshalIndent(v, "  ", "  ")
	//	fmt.Println(string(p))
	//}
	//p.memory.lock.RUnlock()

	//start := time.Now()

	// If we haven't stuck with the same goal long enough, don't re-goal/re-plan
	if time.Since(p.currentGoalSelectedAt) < p.minTimeToRegoal && oldPlan.status() == executionPlanStatusExecuting {
		return oldPlan
	}

	newGoal := p.goalSelector.selectGoal(p.memory)

	// If we arrive at the same goal, and our old plan isn't complete or stale,
	// stick with the plan
	if newGoal == p.currentGoal && oldPlan.status() == executionPlanStatusExecuting && time.Since(p.currentPlanSelectedAt) < p.maxTimeToReplan {
		return oldPlan
	}

	// Otherwise, i.e. if any of these are true:
	// - our old plan is complete
	// - our old plan is stale (exceeds p.maxTimeToReplan)
	// - our new goal != our old goal
	// ... then re-plan
	if newGoal != p.currentGoal {
		fmt.Printf("BRAIN DEBUG: ===== switching goal from %q -> %q =====\n", p.currentGoal, newGoal)
	}
	p.currentGoal = newGoal
	p.currentGoalSelectedAt = time.Now()
	p.currentPlanSelectedAt = time.Now()

	switch p.currentGoal {
	case planGoalDefendSelf:
		plan := &combatPlan{
			actorID:          p.actorID,
			availableActions: allActionsBase,
		}
		plan.plan(p.memory)
		return plan
	case planGoalDoNothing:
		fallthrough
	case planGoalMoveToEmptyLocation:
		fallthrough
	default:
		return &trivialPlan{
			goalName: newGoal,
			memory:   p.memory,
		}
	}
}

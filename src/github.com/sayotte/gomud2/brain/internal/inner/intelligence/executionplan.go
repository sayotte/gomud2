package intelligence

type executionPlan interface {
	status() int
	executeStep(msgSender MessageSender, intellect *Intellect)
}

const (
	executionPlanStatusExecuting = iota
	executionPlanStatusComplete
	executionPlanStatusFailed
)

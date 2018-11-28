package commands

const (
	ErrorNoSuchExit      = "No exit in that direction!"
	ErrorMigrationFailed = "Weird, that didn't seem to work..."
)

var nonFatalErrors = map[string]bool{
	ErrorNoSuchExit:      true,
	ErrorMigrationFailed: true,
}

func IsFatalError(err error) bool {
	return !nonFatalErrors[err.Error()]
}

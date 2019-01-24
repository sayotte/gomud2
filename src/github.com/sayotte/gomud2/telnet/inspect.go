package telnet

import (
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
	"gopkg.in/yaml.v2"
	"strings"
)

const (
	inspectSubcmdLocation = "location"
	inspectSubcmdExits    = "exits"
)

var (
	allInspectSubcommands = []string{
		inspectSubcmdExits,
		inspectSubcmdLocation,
	}
	inspectNoSubcmdErr = fmt.Sprintf("Need one of: %s\n", strings.Join(allInspectSubcommands, ", "))
)

var orderedDirections = []string{
	core.ExitDirectionNorth,
	core.ExitDirectionSouth,
	core.ExitDirectionEast,
	core.ExitDirectionWest,
}

type inspectLocationReport struct {
	ID               uuid.UUID
	Zone             string
	ShortDescription string
	Description      string
}

func (ilr *inspectLocationReport) fromLocation(loc *core.Location) {
	ilr.ID = loc.ID()
	ilr.Zone = loc.Zone().Tag()
	ilr.ShortDescription = loc.ShortDescription()
	ilr.Description = loc.Description()
}

func (ilr inspectLocationReport) bytes() []byte {
	outBytes, err := yaml.Marshal(ilr)
	if err != nil {
		return []byte(fmt.Sprintf("ERROR: yaml.Marshal(): %s\n", err))
	}
	return outBytes
}

type inspectOutExitsReport struct {
	location *core.Location
}

func (ioer inspectOutExitsReport) bytes() []byte {
	directionToExitReportMap := make(map[string]*inspectExitReport)
	for _, exit := range ioer.location.OutExits() {
		exitReport := &inspectExitReport{}
		exitReport.fromExit(exit)
		directionToExitReportMap[exit.Direction()] = exitReport
	}

	var out yaml.MapSlice
	for _, dir := range orderedDirections {
		out = append(out, yaml.MapItem{
			Key:   dir,
			Value: directionToExitReportMap[dir],
		})
	}

	outBytes, err := yaml.Marshal(out)
	if err != nil {
		return []byte(fmt.Sprintf("ERROR: yaml.Marshal(): %s\n", err))
	}
	return outBytes
}

type inspectExitReport struct {
	ID              uuid.UUID
	Zone            string
	Description     string
	Direction       string
	Source          string
	Destination     string
	OtherZoneID     uuid.UUID
	OtherLocationID uuid.UUID
}

func (ixr *inspectExitReport) fromExit(exit *core.Exit) {
	ixr.ID = exit.ID()
	ixr.Zone = exit.Zone().Tag()
	ixr.Description = exit.Description()
	ixr.Direction = exit.Direction()
	if exit.Source() != nil {
		ixr.Source = exit.Source().Tag()
	}
	if exit.Destination() != nil {
		ixr.Destination = exit.Destination().Tag()
	}
	ixr.OtherZoneID = exit.OtherZoneID()
	ixr.OtherLocationID = exit.OtherZoneLocID()
}

func (ixr inspectExitReport) bytes() []byte {
	outBytes, err := yaml.Marshal(ixr)
	if err != nil {
		return []byte(fmt.Sprintf("ERROR: yaml.Marshal(): %s\n", err))
	}
	return outBytes
}

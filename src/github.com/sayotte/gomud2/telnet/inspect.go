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
	inspectSubcmdEdges    = "edges"
)

var (
	allInspectSubcommands = []string{
		inspectSubcmdEdges,
		inspectSubcmdLocation,
	}
	inspectNoSubcmdErr = fmt.Sprintf("Need one of: %s\n", strings.Join(allInspectSubcommands, ", "))
)

var orderedDirections = []string{
	core.EdgeDirectionNorth,
	core.EdgeDirectionSouth,
	core.EdgeDirectionEast,
	core.EdgeDirectionWest,
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

type inspectOutEdgesReport struct {
	location *core.Location
}

func (ioer inspectOutEdgesReport) bytes() []byte {
	directionToEdgeReportMap := make(map[string]*inspectLocationEdgeReport)
	for _, edge := range ioer.location.OutEdges() {
		edgeReport := &inspectLocationEdgeReport{}
		edgeReport.fromLocationEdge(edge)
		directionToEdgeReportMap[edge.Direction()] = edgeReport
	}

	var out yaml.MapSlice
	for _, dir := range orderedDirections {
		out = append(out, yaml.MapItem{
			Key:   dir,
			Value: directionToEdgeReportMap[dir],
		})
	}

	outBytes, err := yaml.Marshal(out)
	if err != nil {
		return []byte(fmt.Sprintf("ERROR: yaml.Marshal(): %s\n", err))
	}
	return outBytes
}

type inspectLocationEdgeReport struct {
	ID              uuid.UUID
	Zone            string
	Description     string
	Direction       string
	Source          string
	Destination     string
	OtherZoneID     uuid.UUID
	OtherLocationID uuid.UUID
}

func (iler *inspectLocationEdgeReport) fromLocationEdge(edge *core.LocationEdge) {
	iler.ID = edge.ID()
	iler.Zone = edge.Zone().Tag()
	iler.Description = edge.Description()
	iler.Direction = edge.Direction()
	if edge.Source() != nil {
		iler.Source = edge.Source().Tag()
	}
	if edge.Destination() != nil {
		iler.Destination = edge.Destination().Tag()
	}
	iler.OtherZoneID = edge.OtherZoneID()
	iler.OtherLocationID = edge.OtherZoneLocID()
}

func (iler inspectLocationEdgeReport) bytes() []byte {
	outBytes, err := yaml.Marshal(iler)
	if err != nil {
		return []byte(fmt.Sprintf("ERROR: yaml.Marshal(): %s\n", err))
	}
	return outBytes
}

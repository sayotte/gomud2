package innerbrain

import (
	"encoding/json"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/commands"
	"time"
)

type jsonString string

func (js jsonString) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(js))
}

//type commaSeparatedStringList string
//
//func (cssl commaSeparatedStringList) push(newEnt string) commaSeparatedStringList {
//	if string(cssl) == "" {
//		return commaSeparatedStringList(newEnt)
//	}
//	return commaSeparatedStringList(string(cssl) + "," + newEnt)
//}
//
//func (cssl commaSeparatedStringList) pop() (string, commaSeparatedStringList) {
//	entries := strings.Split(string(cssl), ",")
//	return entries[len(entries)-1], commaSeparatedStringList(strings.Join(entries[:len(entries)-1], ","))
//}
//
//func (cssl commaSeparatedStringList) remove(remEnt string) commaSeparatedStringList {
//	entries := strings.Split(string(cssl), ",")
//	out := make([]string, 0, len(entries))
//	for _, ent := range entries {
//		if ent == remEnt {
//			continue
//		}
//		out = append(out, ent)
//	}
//	return commaSeparatedStringList(strings.Join(out, ","))
//}
//
//func (cssl commaSeparatedStringList) MarshalJSON() ([]byte, error) {
//	return json.Marshal(string(cssl))
//}
//
//type jsonInt int
//
//func (ji jsonInt) MarshalJSON() ([]byte, error) {
//	return json.Marshal(int(ji))
//}
//
//type jsonFloat64 float64
//
//func (jf jsonFloat64) MarshalJSON() ([]byte, error) {
//	return json.Marshal(float64(jf))
//}
//
//type locationToExitsMap map[uuid.UUID]map[string][2]uuid.UUID
//
//func (ltem locationToExitsMap) MarshalJSON() ([]byte, error) {
//	return json.Marshal(map[uuid.UUID]map[string][2]uuid.UUID(ltem))
//}

type jsonZoneInfoMap map[uuid.UUID]map[uuid.UUID]locInfoEntry

func (jzim jsonZoneInfoMap) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[uuid.UUID]map[uuid.UUID]locInfoEntry(jzim))
}

type locInfoEntry struct {
	Info      commands.LocationInfo
	Timestamp time.Time
}

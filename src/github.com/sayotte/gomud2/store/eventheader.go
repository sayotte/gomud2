package store

import (
	"encoding/binary"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
	"time"
)

const eventHeaderByteLen = 48

func eventHeaderFromDomainEvent(from core.Event) eventHeader {
	return eventHeader{
		EventType:      from.Type(),
		Timestamp:      from.Timestamp(),
		Version:        from.Version(),
		AggregateId:    from.AggregateId(),
		SequenceNumber: from.SequenceNumber(),
	}
}

type eventHeader struct {
	EventType      int
	Timestamp      time.Time
	Version        int
	AggregateId    uuid.UUID
	SequenceNumber uint64
	Length         int
	UseCompression bool
}

func (eh eventHeader) MarshalBinary() ([]byte, error) {
	//aggID [16]byte // 16
	//seqNum uint64  // 8
	//len uint32     // 4
	//typ uint16     // 2
	//ver uint16     // 2
	//time [15]byte  // 15
	//compress       // 1
	//// total of 48 bytes

	buf := make([]byte, eventHeaderByteLen)
	copy(buf[0:16], eh.AggregateId.Bytes())
	binary.LittleEndian.PutUint64(buf[16:24], eh.SequenceNumber)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(eh.Length))
	binary.LittleEndian.PutUint16(buf[28:30], uint16(eh.EventType))
	binary.LittleEndian.PutUint16(buf[30:32], uint16(eh.Version))
	timeBytes, err := eh.Timestamp.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("Time.MarshalBinary(): %s", err)
	}
	copy(buf[32:47], timeBytes)
	var useCompression byte
	if eh.UseCompression {
		useCompression = 1
	} else {
		useCompression = 0
	}
	buf[47] = useCompression
	return buf, nil
}

func (eh *eventHeader) UnmarshalBinary(buf []byte) error {
	if len(buf) != eventHeaderByteLen {
		return fmt.Errorf("need a fixed-length %d-byte buffer, got %d bytes", eventHeaderByteLen, len(buf))
	}
	copy(eh.AggregateId[:], buf[:16])
	eh.SequenceNumber = binary.LittleEndian.Uint64(buf[16:24])
	eh.Length = int(binary.LittleEndian.Uint32(buf[24:28]))
	eh.EventType = int(binary.LittleEndian.Uint16(buf[28:30]))
	eh.Version = int(binary.LittleEndian.Uint16(buf[30:32]))
	err := (&eh.Timestamp).UnmarshalBinary(buf[32:47])
	if err != nil {
		return fmt.Errorf("Time.UnmarshalBinary(): %s", err)
	}
	useCompression := buf[47]
	if useCompression == 0 {
		eh.UseCompression = false
	} else {
		eh.UseCompression = true
	}
	return nil
}

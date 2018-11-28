package store

import (
	"encoding/binary"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/domain"
)

const eventHeaderByteLen = 33

func eventHeaderFromDomainEvent(from domain.Event) eventHeader {
	return eventHeader{
		EventType:      from.Type(),
		Version:        from.Version(),
		AggregateId:    from.AggregateId(),
		SequenceNumber: from.SequenceNumber(),
	}
}

type eventHeader struct {
	EventType      int
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
	//compress       // 1
	//// total of 33 bytes

	buf := make([]byte, eventHeaderByteLen)
	copy(buf[0:16], eh.AggregateId.Bytes())
	binary.LittleEndian.PutUint64(buf[16:24], eh.SequenceNumber)
	binary.LittleEndian.PutUint32(buf[24:28], uint32(eh.Length))
	binary.LittleEndian.PutUint16(buf[28:30], uint16(eh.EventType))
	binary.LittleEndian.PutUint16(buf[30:32], uint16(eh.Version))
	var useCompression byte
	if eh.UseCompression {
		useCompression = 1
	} else {
		useCompression = 0
	}
	buf[32] = useCompression
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
	useCompression := buf[32]
	if useCompression == 0 {
		eh.UseCompression = false
	} else {
		eh.UseCompression = true
	}
	return nil
}

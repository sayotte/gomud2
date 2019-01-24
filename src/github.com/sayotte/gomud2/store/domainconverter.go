package store

import (
	"bytes"
	"compress/flate"
	"encoding/json"
	"fmt"
	"github.com/sayotte/gomud2/core"
	"io"
	"io/ioutil"
)

type FromDomainer interface {
	FromDomain(core.Event)
	Header() eventHeader
}

type ToDomainer interface {
	ToDomain() core.Event
	SetHeader(eventHeader)
}

func writeEvent(e core.Event, outStream io.Writer, useCompression bool) error {
	var frommer FromDomainer
	switch e.Type() {
	case core.EventTypeActorAddToZone:
		frommer = &actorAddToZoneEvent{}
	case core.EventTypeActorMove:
		frommer = &actorMoveEvent{}
	case core.EventTypeActorRemoveFromZone:
		frommer = &actorRemoveFromZoneEvent{}
	case core.EventTypeLocationAddToZone:
		frommer = &locationAddToZoneEvent{}
	case core.EventTypeLocationUpdate:
		frommer = &locationUpdateEvent{}
	case core.EventTypeLocationEdgeAddToZone:
		frommer = &locationEdgeAddToZoneEvent{}
	case core.EventTypeLocationEdgeUpdate:
		frommer = &locationEdgeUpdateEvent{}
	case core.EventTypeObjectAddToZone:
		frommer = &objectAddToZoneEvent{}
	case core.EventTypeObjectRemoveFromZone:
		frommer = &objectRemoveFromZoneEvent{}
	case core.EventTypeObjectMove:
		frommer = &objectMoveEvent{}
	default:
		return fmt.Errorf("unhandled event type %T", e)
	}
	frommer.FromDomain(e)

	bodyBytes, err := json.Marshal(frommer)
	if err != nil {
		return fmt.Errorf("json.Marshal(frommer): %s", err)
	}
	if useCompression {
		compressedBuf := &bytes.Buffer{}
		flateWriter, err := flate.NewWriter(compressedBuf, -1)
		if err != nil {
			return fmt.Errorf("flate.NewWriter():%s ", err)
		}
		_, err = io.Copy(flateWriter, bytes.NewReader(bodyBytes))
		if err != nil {
			return fmt.Errorf("io.Copy(flateWriter, bodyBytes): %s", err)
		}
		err = flateWriter.Close()
		if err != nil {
			return fmt.Errorf("flateWriter.Close(): %s", err)
		}
		bodyBytes = compressedBuf.Bytes()
	}
	header := frommer.Header()
	header.Length = len(bodyBytes)
	header.UseCompression = useCompression
	headerBytes, err := header.MarshalBinary()
	if err != nil {
		return fmt.Errorf("header.MarshalBinary(): %s", err)
	}

	_, err = outStream.Write(headerBytes)
	if err != nil {
		return fmt.Errorf("outStream.Write(headerBytes): %s", err)
	}
	_, err = outStream.Write(bodyBytes)
	if err != nil {
		return fmt.Errorf("outStream.Write(bodyBytes): %s", err)
	}
	return nil
}

func readEvent(inStream io.Reader) (core.Event, error) {
	buf, err := ioutil.ReadAll(io.LimitReader(inStream, eventHeaderByteLen))
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadAll(header): %s", err)
	}
	// ioutil.ReadAll won't return io.EOF, so we have to check for a
	// 0-byte buffer being returned
	if len(buf) == 0 {
		return nil, io.EOF
	}
	var hdr eventHeader
	err = (&hdr).UnmarshalBinary(buf)
	if err != nil {
		return nil, fmt.Errorf("hdr.UnmarshalBinary(): %s", err)
	}
	var bodyReader io.Reader
	if hdr.UseCompression {
		bodyReader = flate.NewReader(io.LimitReader(inStream, int64(hdr.Length)))
	} else {
		bodyReader = io.LimitReader(inStream, int64(hdr.Length))
	}
	buf, err = ioutil.ReadAll(bodyReader)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadAll(body): %s", err)
	}
	var toEr ToDomainer
	switch hdr.EventType {
	case core.EventTypeActorAddToZone:
		toEr = &actorAddToZoneEvent{}
	case core.EventTypeActorMove:
		toEr = &actorMoveEvent{}
	case core.EventTypeActorRemoveFromZone:
		toEr = &actorRemoveFromZoneEvent{}
	case core.EventTypeLocationAddToZone:
		toEr = &locationAddToZoneEvent{}
	case core.EventTypeLocationUpdate:
		toEr = &locationUpdateEvent{}
	case core.EventTypeLocationEdgeAddToZone:
		toEr = &locationEdgeAddToZoneEvent{}
	case core.EventTypeLocationEdgeUpdate:
		toEr = &locationEdgeUpdateEvent{}
	case core.EventTypeObjectAddToZone:
		toEr = &objectAddToZoneEvent{}
	case core.EventTypeObjectRemoveFromZone:
		toEr = &objectRemoveFromZoneEvent{}
	case core.EventTypeObjectMove:
		toEr = &objectMoveEvent{}
	}
	err = json.Unmarshal(buf, toEr)
	if err != nil {
		return nil, fmt.Errorf("json.Unmarshal(): %s", err)
	}
	toEr.SetHeader(hdr)
	return toEr.ToDomain(), nil
}

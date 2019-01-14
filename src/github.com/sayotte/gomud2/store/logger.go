package store

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
	uuid2 "github.com/sayotte/gomud2/uuid"
	"io"
	"io/ioutil"
	"os"
)

// IntentLogger is a change-intent-log, useful for implementing transactional
// semantics for a higher-level store. By first making a record of the
// changes you intend to make (and optionally a record of how to un-do those
// changes), and later making a record of the successful completion of those
// changes, IntentLogger has enough information to later
// detect and give you context for any changes that weren't complete at the
// time of a crash.
//
// Normal use works like this:
//
// 1- create a new log using IntentLogger.Open()
//
// 2- write an intent using IntentLogger.WriteIntent(); this returns a
// transaction ID you must store
//
// 3- execute your intended changes as usual
//
// 4- confirm the original intent has been completed using
// IntentLogger.ConfirmIntentCompletion()
//
// 5- (optional) close the log using IntentLogger.Close()
//
// 6- on re-start, call IntentLogger.Open() again; this time, the callback func you
// provide will be called for each intent that was successfully written to the
// log but never marked as complete later on
//
//
// Some notes on semantics:
//
// * WriteIntent() and ConfirmIntentCompletion() call io.Sync() before
// returning.
//
// * After each successful call to the handler provided to
// Open(), it automatically calls ConfirmIntentCompletion().
//
// * Open() will truncate any trailing+incomplete
// entries it finds while reading the input stream.
type IntentLogger struct {
	Filename string
	fd       *os.File
}

// WriteIntent writes the given redo/undo content to the log, and returns
// an ID for that entry which must later be passed to ConfirmIntentCompletion()
// to mark the entry as completed.
func (il IntentLogger) WriteIntent(redo, undo []core.Event) (uuid.UUID, error) {
	if il.fd == nil {
		return uuid.Nil, errors.New("IntentLogger.WriteIntent on non-open IntentLogger")
	}
	redoBuf := &bytes.Buffer{}
	for _, e := range redo {
		err := writeEvent(e, redoBuf, true)
		if err != nil {
			return uuid.Nil, err
		}
	}
	undoBuf := &bytes.Buffer{}
	for _, e := range undo {
		err := writeEvent(e, undoBuf, true)
		if err != nil {
			return uuid.Nil, err
		}
	}

	we := logEntry{
		TransactionID:        uuid2.NewId(),
		TransactionCompleted: false,
		RedoContent:          redoBuf.Bytes(),
		UndoContent:          undoBuf.Bytes(),
	}
	err := il.writeEntry(we)
	if err != nil {
		return uuid.Nil, err
	}
	return we.TransactionID, nil
}

// ConfirmIntentCompletion writes a completion-marker for the intent-entry
// with the given ID.
func (il IntentLogger) ConfirmIntentCompletion(transactionID uuid.UUID) error {
	if il.fd == nil {
		return errors.New("IntentLogger.ConfirmIntentCompletion on non-open IntentLogger")
	}
	we := logEntry{
		TransactionID:        transactionID,
		TransactionCompleted: true,
	}
	err := il.writeEntry(we)
	if err != nil {
		return err
	}
	return nil
}

func (il IntentLogger) writeEntry(le logEntry) error {
	err := le.serialize(il.fd)
	if err != nil {
		return err
	}
	err = il.fd.Sync()
	if err != nil {
		return fmt.Errorf("il.Stream.Sync(): %s", err)
	}
	return nil
}

// Open opens and replays IntentLogger.Filename.
//
// For each intent-entry, it searches for a completion marker. For all entries
// with no corresponding completion marker, it calls the provided
// IncompleteTransactionHandler with the redo / undo []byte content provided to
// the original intent-entry. After calling the handler, and if the handler did
// not return an error, it writes a new completion-marker to the log to prevent
// the same entry from being handled if IntentLogger.Open() is called multiple times
// on the same logfile.
func (il *IntentLogger) Open(handler func(redo, undo []core.Event) error) error {
	if il.fd != nil {
		return errors.New("IntentLogger.Open() on already-open IntentLogger")
	}
	var err error
	il.fd, err = os.OpenFile(il.Filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("os.OpenFile(%q, ...): %s", il.Filename, err)
	}

	transMap := make(map[uuid.UUID]logEntry)
	var orderedTransIDs uuid2.UUIDList
	for {
		offsetBeforeReading, err := il.fd.Seek(0, 1)
		if err != nil {
			return fmt.Errorf("il.fd.Seek(0, 1): %s", err)
		}
		we := logEntry{}
		err = (&we).deserialize(il.fd)
		if err != nil {
			if err == io.EOF {
				// If we hit EOF without successfully reading a full record,
				// then we need to purge any partially-written intents we
				// found.
				err = il.fd.Truncate(offsetBeforeReading)
				if err != nil {
					return fmt.Errorf("il.Stream.Truncate(truncOffset): %s", err)
				}
				// Also, seek back to the correct beginning of the next entry,
				// as we'll need to write more further down in the function.
				_, err = il.fd.Seek(offsetBeforeReading, 0)
				if err != nil {
					return fmt.Errorf("il.fd.Seek(offsetBeforeReading, 0): %s", err)
				}
				// Ok, that done, we should now return since hitting EOF was
				// expected, and we still need to handle any incomplete
				// transactions.
				break
			}
			return err
		}

		if we.TransactionCompleted {
			delete(transMap, we.TransactionID)
			orderedTransIDs = orderedTransIDs.Remove(we.TransactionID)
			continue
		}
		orderedTransIDs = append(orderedTransIDs, we.TransactionID)
		transMap[we.TransactionID] = we
	}

	// We get to here when we hit EOF; anything left in orderedTransIDs is an
	// incomplete transaction, so process those in-order.
	for _, id := range orderedTransIDs {
		// first, call the handler to take whatever action is needed
		we := transMap[id]
		if handler == nil {
			return errors.New("IntentLogger.Open() needs to call handler, but handler is nil")
		}
		var redoEvents []core.Event
		redoBuf := bytes.NewBuffer(we.RedoContent)
		for {
			newRedo, err := readEvent(redoBuf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			redoEvents = append(redoEvents, newRedo)
		}
		var undoEvents []core.Event
		undoBuf := bytes.NewBuffer(we.UndoContent)
		for {
			newRedo, err := readEvent(undoBuf)
			if err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			undoEvents = append(undoEvents, newRedo)
		}
		err = handler(redoEvents, undoEvents)
		if err != nil {
			return err
		}
		// now, write a completion log so we (hopefully) don't end up
		// replaying this entry again the next time this function is called
		err = il.ConfirmIntentCompletion(id)
		if err != nil {
			return err
		}
	}

	return nil
}

// Close calls os.File.Close() and resets internal state to that of a new
// IntentLogger.
func (il *IntentLogger) Close() {
	_ = il.fd.Close()
	il.fd = nil
}

type logEntry struct {
	TransactionID        uuid.UUID
	TransactionCompleted bool
	RedoContent          []byte
	UndoContent          []byte
}

func (le logEntry) serialize(out io.Writer) error {
	hdrBytes, _ := le.genHeader().MarshalBinary()
	_, err := out.Write(hdrBytes)
	if err != nil {
		return fmt.Errorf("out.Write(hdrBytes): %s", err)
	}

	if len(le.RedoContent) > 0 {
		_, err = out.Write(le.RedoContent)
		if err != nil {
			return fmt.Errorf("out.Write(le.RedoContent): %s", err)
		}
	}
	if len(le.UndoContent) > 0 {
		_, err = out.Write(le.UndoContent)
		if err != nil {
			return fmt.Errorf("out.Write(le.UndoContent): %s", err)
		}
	}

	return nil
}

func (le logEntry) genHeader() logEntrySerializeHeader {
	return logEntrySerializeHeader{
		TransactionID:        le.TransactionID,
		RedoContentLen:       uint32(len(le.RedoContent)),
		UndoContentLen:       uint32(len(le.UndoContent)),
		TransactionCompleted: le.TransactionCompleted,
	}
}

func (le *logEntry) deserialize(in io.Reader) error {
	hdrBytes, err := ioutil.ReadAll(io.LimitReader(in, int64(logEntrySerializeHeaderLength)))
	if len(hdrBytes) < logEntrySerializeHeaderLength {
		return io.EOF
	}
	if err != nil {
		return fmt.Errorf("ioutil.ReadAll(io.LimitReader(in, int64(logEntrySerializeHeaderLength))): %s", err)
	}
	hdr := &logEntrySerializeHeader{}
	err = hdr.UnmarshalBinary(hdrBytes)
	if err != nil {
		return err
	}
	le.TransactionID = hdr.TransactionID
	le.TransactionCompleted = hdr.TransactionCompleted

	if hdr.RedoContentLen > 0 {
		le.RedoContent, err = ioutil.ReadAll(io.LimitReader(in, int64(hdr.RedoContentLen)))
		if len(le.RedoContent) < int(hdr.RedoContentLen) {
			return io.EOF
		}
		if err != nil {
			return fmt.Errorf("ioutil.ReadAll(io.LimitReader(in, int64(hdr.RedoContentLen))): %s", err)
		}
	}

	if hdr.UndoContentLen > 0 {
		le.UndoContent, err = ioutil.ReadAll(io.LimitReader(in, int64(hdr.UndoContentLen)))
		if len(le.UndoContent) < int(hdr.UndoContentLen) {
			return io.EOF
		}
		if err != nil {
			return fmt.Errorf("ioutil.ReadAll(io.LimitReader(in, int64(hdr.UndoContentLen))): %s", err)
		}
	}

	return nil
}

const logEntrySerializeHeaderLength = 32

type logEntrySerializeHeader struct {
	TransactionID        uuid.UUID // 16
	RedoContentLen       uint32    // 4
	UndoContentLen       uint32    // 4
	TransactionCompleted bool      // 1
	// imaginary field handled by the [Un]MarshalBinary() routines,
	// gives us a 64-bit word-aligned struct for easier hexdump reading
	//Padding [7]byte // 7

	// total length: 32 bytes
}

func (lesh logEntrySerializeHeader) MarshalBinary() ([]byte, error) {
	outBuf := make([]byte, logEntrySerializeHeaderLength)
	for i := range outBuf {
		outBuf[i] = 0
	}
	copy(outBuf, lesh.TransactionID[:])
	binary.LittleEndian.PutUint32(outBuf[16:20], lesh.RedoContentLen)
	binary.LittleEndian.PutUint32(outBuf[20:24], lesh.UndoContentLen)
	if lesh.TransactionCompleted {
		outBuf[24] = byte(1)
	}

	return outBuf, nil
}

func (lesh *logEntrySerializeHeader) UnmarshalBinary(inBuf []byte) error {
	if len(inBuf) != logEntrySerializeHeaderLength {
		return fmt.Errorf("expected input-buffer of %d bytes, got %d bytes", logEntrySerializeHeaderLength, len(inBuf))
	}
	copy(lesh.TransactionID[:], inBuf)
	lesh.RedoContentLen = binary.LittleEndian.Uint32(inBuf[16:20])
	lesh.UndoContentLen = binary.LittleEndian.Uint32(inBuf[20:24])
	if inBuf[24] == 1 {
		lesh.TransactionCompleted = true
	}

	return nil
}

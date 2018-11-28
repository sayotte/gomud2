package store

import (
	uuid2 "github.com/sayotte/gomud2/uuid"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLogger_Open(t *testing.T) {
	redoContent := []byte("redoredo") // 8-byte strings make for nice hexdumps
	undoContent := []byte("undoundo")
	l := &IntentLogger{
		Filename: filepath.Join(os.TempDir(), "TestLogger_Open"),
	}
	defer os.Remove(l.Filename)
	var callCount int
	handler := func(redo, undo []byte) error {
		callCount++
		return nil
	}

	// basic happy path
	err := l.Open(handler)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if callCount > 0 {
		t.Errorf("expected 0 calls to handler on IntentLogger.Open(), got %d", callCount)
	}
	id1, err := l.WriteIntent(redoContent, undoContent)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	err = l.ConfirmIntentCompletion(id1)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	err = l.Close()
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	callCount = 0
	err = l.Open(handler)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if callCount > 0 {
		t.Errorf("expected 0 calls to handler on IntentLogger.Open(), got %d", callCount)
	}

	// incomplete transaction; we expect l.Open() to call our handler
	_, err = l.WriteIntent(redoContent, undoContent)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	err = l.Close()
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	callCount = 0
	err = l.Open(handler)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call to handler on IntentLogger.Open(), got %d", callCount)
	}

	// replay the same log again now; we should get 0 calls since a completion
	// record should've been written after our handler was called
	err = l.Close()
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	callCount = 0
	err = l.Open(handler)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if callCount != 0 {
		t.Errorf("expected 0 calls to handler on IntentLogger.Open(), got %d", callCount)
	}

	// now write an incomplete entry-- a header missing the expected body
	// then call l.Open(), which should truncate the incomplete entry
	hdr := logEntrySerializeHeader{
		TransactionID:  uuid2.NewId(),
		RedoContentLen: 32,
	}
	hdrBytes, _ := hdr.MarshalBinary()
	_, err = l.fd.Write(hdrBytes)
	if err != nil {
		t.Fatalf("l.fd.Write(hdrBytes): %s", err)
	}
	err = l.fd.Sync()
	if err != nil {
		t.Fatalf("l.fd.Sync(): %s", err)
	}
	currPosition, err := l.fd.Seek(0, 1)
	expectedLength := currPosition - logEntrySerializeHeaderLength
	if err != nil {
		t.Fatalf("l.fd.Seek(0, 1): %s", err)
	}
	err = l.Close()
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	err = l.Open(nil)
	if err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	newPosition, err := l.fd.Seek(0, 2)
	if err != nil {
		t.Fatalf("l.fd.Seek(0, 2): %s", err)
	}
	if newPosition != expectedLength {
		t.Errorf("expected file to be truncated to %d bytes, but it's still %d bytes", expectedLength, newPosition)
	}
}

func TestLogEntry_roundtrip(t *testing.T) {
	out := make([]*logEntry, 3)
	out[0] = &logEntry{
		TransactionID:        uuid2.NewId(),
		TransactionCompleted: false,
		RedoContent:          []byte("redoredo"),
		UndoContent:          []byte("undoundo"),
	}
	out[1] = &logEntry{
		TransactionID:        out[0].TransactionID,
		TransactionCompleted: true,
	}
	out[2] = out[0]

	outFile := filepath.Join(os.TempDir(), "TestLogEntry_roundtrip")
	fd, err := os.Create(outFile)
	if err != nil {
		t.Fatalf("os.Create(outFile): %s", err)
	}
	defer os.Remove(outFile)
	for i := range out {
		err := out[i].serialize(fd)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}
	}

	err = fd.Sync()
	if err != nil {
		t.Fatalf("fd.Sync(): %s", err)
	}
	_, err = fd.Seek(0, 0)
	if err != nil {
		t.Fatalf("fd.Seek(0, 0): %s", err)
	}

	for i := range out {
		inx := &logEntry{}
		err := inx.deserialize(fd)
		if err != nil {
			t.Errorf("unexpected error: %s", err)
		}

		if !reflect.DeepEqual(out[i], inx) {
			t.Errorf("[%d]: %v != %v", i, out[i], inx)
		}
	}
}

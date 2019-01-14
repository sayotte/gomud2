package store

import (
	"fmt"
	"github.com/satori/go.uuid"
	"github.com/sayotte/gomud2/core"
	"github.com/sayotte/gomud2/rpc"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

type EventStore struct {
	Filename          string
	UseCompression    bool
	SnapshotDirectory string
	outStream         io.Writer
}

func (es *EventStore) PersistEvent(e core.Event) error {
	if es.outStream == nil {
		dir := filepath.Dir(es.Filename)
		if !pathExists(dir) {
			err := os.MkdirAll(dir, 0755)
			if err != nil {
				return fmt.Errorf("os.MkdirAll(%q, 0755): %s", dir, err)
			}
		}

		fd, err := os.OpenFile(
			es.Filename,
			os.O_CREATE|os.O_APPEND|os.O_WRONLY,
			0644,
		)
		if err != nil {
			return fmt.Errorf("os.OpenFile(%q, ...): %s", es.Filename, err)
		}
		es.outStream = fd
	}

	return writeEvent(e, es.outStream, es.UseCompression)
}

func (es *EventStore) RetrieveAll() (<-chan rpc.Response, error) {
	return es.retrieveAllFromFile(es.Filename)
}

func (es *EventStore) retrieveAllFromFile(filename string) (<-chan rpc.Response, error) {
	var inStream io.ReadCloser
	inStream, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("os.Open(%q): %s", filename, err)
	}
	inOutChan := make(chan rpc.Response, 20)
	go func(outChan chan<- rpc.Response) {
		defer func() {
			_ = inStream.Close()
		}()
		for {
			e, err := readEvent(inStream)
			if err != nil {
				if err != io.EOF {
					res := rpc.Response{
						Err: err,
					}
					outChan <- res
				}
				close(outChan)
				return
			}
			res := rpc.Response{
				Value: e,
			}
			outChan <- res
		}
	}(inOutChan)
	return inOutChan, nil
}

func (es *EventStore) RetrieveAllForZone(zoneID uuid.UUID) (<-chan rpc.Response, error) {
	return es.RetrieveUpToSequenceNumsForZone(math.MaxUint64, zoneID)
}

func (es *EventStore) RetrieveUpToSequenceNumsForZone(endNum uint64, zoneID uuid.UUID) (<-chan rpc.Response, error) {
	retChan := make(chan rpc.Response)
	var startNum uint64
	// first check for a snapshot, and return that if possible
	snapChan, snapSeqNum, err := es.findPreviousSnapshotForZone(endNum, zoneID)
	if err != nil {
		return nil, err
	}
	if snapChan != nil {
		startNum = snapSeqNum + 1
	}

	// super naive implementation, we just wrap RetrieveAll()
	// and filter the entire set
	srcChan, err := es.RetrieveAll()
	if err != nil {
		return nil, err
	}
	go func(snapChan, srcChan <-chan rpc.Response, startNumInner, endNumInner uint64, outChan chan<- rpc.Response) {
		// replay snapshot if we were given one
		if snapChan != nil {
			for res := range snapChan {
				outChan <- res
			}
		}
		// replay all other events after
		for res := range srcChan {
			if res.Err != nil {
				outChan <- res
				close(outChan)
				return
			}
			e := res.Value.(core.Event)
			if e.AggregateId() != zoneID {
				continue
			}
			if e.SequenceNumber() < startNumInner {
				continue
			}
			if e.SequenceNumber() > endNumInner {
				close(outChan)
				return
			}
			outChan <- res
		}
		close(outChan)
	}(snapChan, srcChan, startNum, endNum, retChan)
	return retChan, nil
}

func (es *EventStore) PersistSnapshot(zoneID uuid.UUID, seqNum uint64, snapEvents []core.Event) error {
	if _, err := os.Stat(es.SnapshotDirectory); os.IsNotExist(err) {
		err = os.MkdirAll(es.SnapshotDirectory, 0755)
		if err != nil {
			return fmt.Errorf("os.MkdirAll(%q): %s", es.SnapshotDirectory, err)
		}
	}

	filename := fmt.Sprintf("%s/%s_%d.dat", es.SnapshotDirectory, zoneID, seqNum)
	fd, err := os.OpenFile(filename, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("os.OpenFile(%q, ...): %s", filename, err)
	}
	defer fd.Close()

	for _, snapEvent := range snapEvents {
		err = writeEvent(snapEvent, fd, es.UseCompression)
		if err != nil {
			return err
		}
	}
	return nil
}

func (es *EventStore) findPreviousSnapshotForZone(maxSeqNum uint64, zoneID uuid.UUID) (<-chan rpc.Response, uint64, error) {
	if !pathExists(es.SnapshotDirectory) {
		err := os.MkdirAll(es.SnapshotDirectory, 0755)
		if err != nil {
			return nil, 0, fmt.Errorf("os.MkdirAll(%q, 0755): %s", es.SnapshotDirectory, err)
		}
	}
	fInfos, err := ioutil.ReadDir(es.SnapshotDirectory)
	if err != nil {
		return nil, 0, fmt.Errorf("ioutil.ReadDir(%q): %s", es.SnapshotDirectory, err)
	}

	matchRE := regexp.MustCompile(fmt.Sprintf(`^%s_(\d+)\.dat`, zoneID.String()))
	var highestSeqNum uint64
	var selectedFileName string
	for _, fi := range fInfos {
		matches := matchRE.FindStringSubmatch(fi.Name())
		if matches == nil {
			continue
		}
		seqNum, err := strconv.ParseUint(matches[1], 0, 64)
		if err != nil {
			return nil, 0, fmt.Errorf("strconv.ParseUint(%q, 0, 64): %s", matches[1], err)
		}
		if seqNum >= highestSeqNum && seqNum <= maxSeqNum {
			highestSeqNum = seqNum
			selectedFileName = fi.Name()
		}
	}
	if selectedFileName == "" {
		return nil, 0, nil
	}

	outChan, err := es.retrieveAllFromFile(filepath.Join(es.SnapshotDirectory, selectedFileName))
	if err != nil {
		return nil, 0, err
	}
	return outChan, highestSeqNum, nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		return false
	}
	return true
}

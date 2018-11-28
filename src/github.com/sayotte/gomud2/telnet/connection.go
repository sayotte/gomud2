package telnet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

type connectionControlMessage struct {
	messageType string
	messageBody interface{}
}

const (
	controlMessageTypeError             = "error"
	controlMessageTypeWindowSizeChanged = "window-size-changed"
	controlMessageTypeTerminalType      = "terminal-type"
	controlMessageTypeConnectionClosed  = "connection-closed"
)

const (
	telnetIAC                = 255 // interpret as command
	telnetWILL               = 251
	telnetWONT               = 252
	telnetDO                 = 253
	telnetDONT               = 254
	telnetOptionTerminalType = 24
	telnetOptionNAWS         = 31  // NAWS == negotiate a window size
	telnetSB                 = 250 // subnegotiation begin
	telnetSE                 = 240 // subnegotiation end
)

func newLineBufferedConnection(conn net.Conn, queueLen int) *lineBufferedConnection {
	return &lineBufferedConnection{
		conn:         conn,
		shutdownOnce: &sync.Once{},
		rxChan:       make(chan []byte, queueLen),
		txChan:       make(chan []byte, queueLen),
		ctrlMsgChan:  make(chan connectionControlMessage),
		stopChan:     make(chan struct{}),
	}
}

type lineBufferedConnection struct {
	conn         net.Conn
	shutdownOnce *sync.Once
	rxChan       chan []byte
	txChan       chan []byte
	ctrlMsgChan  chan connectionControlMessage
	stopChan     chan struct{}
}

func (c *lineBufferedConnection) Start() error {
	if c.conn == nil {
		return errors.New("uninitialized, must create lineBufferedConnection using constructor")
	}
	err := c.initNAWS()
	if err != nil {
		return err
	}
	err = c.initTerminalType()
	if err != nil {
		return err
	}
	go c.gatherDataLoop()
	go c.sendDataLoop()
	return nil
}

// initNAWS initiates window-size negotiation with the telnet client.
// It sends a "DO NAWS" suggestion to the client, which we expect (but do not require)
// them to respond to with first a "WILL NAWS" and then a "SB NAWS <width> <height>"
// response code.
func (c *lineBufferedConnection) initNAWS() error {
	initiateNAWS := []byte{
		telnetIAC,
		telnetDO,
		telnetOptionNAWS,
	}
	_, err := c.conn.Write(initiateNAWS)
	if err != nil {
		return fmt.Errorf("conn.Write(): %s", err)
	}
	return nil
}

func (c *lineBufferedConnection) initTerminalType() error {
	initiateTermType := []byte{
		telnetIAC,
		telnetDO,
		telnetOptionTerminalType,
	}
	_, err := c.conn.Write(initiateTermType)
	if err != nil {
		return fmt.Errorf("conn.Write(): %s", err)
	}
	return nil
}

func (c *lineBufferedConnection) Shutdown() {
	c.shutdownOnce.Do(func() {
		close(c.stopChan)
		_ = c.conn.Close() // ignore the possible error
	})
}

func (c *lineBufferedConnection) RxChan() <-chan []byte {
	return c.rxChan
}

func (c *lineBufferedConnection) ControlChan() <-chan connectionControlMessage {
	return c.ctrlMsgChan
}

func (c *lineBufferedConnection) Send(msg []byte) {
	c.txChan <- msg
}

func (c *lineBufferedConnection) sendDataLoop() {
	for {
		select {
		case <-c.stopChan:
			return
		case newBytes := <-c.txChan:
			for idx := 0; idx < len(newBytes)-1; {
				bytesWritten, err := c.conn.Write(newBytes[idx:])
				idx += bytesWritten
				if err != nil {
					ctrlMsg := connectionControlMessage{
						messageType: controlMessageTypeError,
						messageBody: fmt.Errorf("while sending data: %s", err),
					}
					c.ctrlMsgChan <- ctrlMsg
					c.Shutdown()
					return
				}
			}
		}
	}
}

// read inspects incoming data for telnet command codes; if it finds them, it
// interprets them (or throws an error if it doesn't know how), then strips
// them out of the buffer before returning the modified buffer / byte count.
func (c *lineBufferedConnection) readTelnet(buf []byte) (int, error) {
	bytesRead, err := c.conn.Read(buf)
	if err != nil {
		return bytesRead, err
	}
	bytesStripped, err := c.findAndStripTelnetCodes(buf)
	bytesRead -= bytesStripped
	return bytesRead, err
}

func (c *lineBufferedConnection) findAndStripTelnetCodes(buf []byte) (int, error) {
	bytesConsumed := 0
	for {
		iacIdx := bytes.Index(buf, []byte{telnetIAC})
		if iacIdx == -1 {
			return bytesConsumed, nil
		}
		switch buf[iacIdx+1] {
		case telnetWILL:
			//fmt.Printf("DEBUG: received telnet WILL with code %d\n", buf[iacIdx+2])
			switch buf[iacIdx+2] {
			case telnetOptionTerminalType:
				requestTermType := []byte{
					telnetIAC,
					telnetSB,
					telnetOptionTerminalType,
					1,
					telnetIAC,
					telnetSE,
				}
				_, err := c.conn.Write(requestTermType)
				if err != nil {
					return bytesConsumed, fmt.Errorf("conn.Write(): %s", err)
				}
			}
			copy(buf[iacIdx:], buf[iacIdx+3:]) // strip out the WILL code
			bytesConsumed += 3
		case telnetWONT:
			//fmt.Printf("DEBUG: received telnet WONT with code %d\n", buf[iacIdx+2])
			copy(buf[iacIdx:], buf[iacIdx+3:]) // strip out the WONT code
			bytesConsumed += 3
		case telnetSB:
			switch buf[iacIdx+2] {
			case telnetOptionNAWS:
				terminalWidth := binary.BigEndian.Uint16(buf[iacIdx+3 : iacIdx+5])
				terminalHeight := binary.BigEndian.Uint16(buf[iacIdx+5 : iacIdx+7])
				ctrlMsg := connectionControlMessage{
					messageType: controlMessageTypeWindowSizeChanged,
					messageBody: [2]uint16{terminalWidth, terminalHeight},
				}
				c.ctrlMsgChan <- ctrlMsg
				copy(buf[iacIdx:], buf[iacIdx+9:]) // consumes trailing IAC+SE
				bytesConsumed += 9
			case telnetOptionTerminalType:
				termStringLen := bytes.Index(buf[iacIdx+4:], []byte{telnetIAC})
				termString := string(buf[iacIdx+4 : iacIdx+4+termStringLen])
				ctrlMsg := connectionControlMessage{
					messageType: controlMessageTypeTerminalType,
					messageBody: termString,
				}
				c.ctrlMsgChan <- ctrlMsg
				copy(buf[iacIdx:], buf[iacIdx+6+termStringLen:])
				bytesConsumed += 6 + termStringLen
			default:
				return bytesConsumed, fmt.Errorf("unrecognized telnet sub-negotiation type %d", buf[iacIdx+2])
			}
		}
	}
}

func (c *lineBufferedConnection) gatherDataLoop() {
	buf := make([]byte, 4096)
	var nextIdx int
	for {
		select {
		case <-c.stopChan:
			close(c.rxChan)
			return
		default:
		}

		buf = buf[:4096] // resize buf to full capacity
		bytesRead, err := c.readTelnet(buf)
		nextIdx += bytesRead

		// handle data returned first
		newLines, bytesConsumed := findLines(buf[:nextIdx])
		for _, line := range newLines {
			c.rxChan <- line
		}
		nextIdx -= bytesConsumed

		// handle errors last
		if err != nil {
			var ctrlMsg connectionControlMessage
			if err == io.EOF {
				ctrlMsg = connectionControlMessage{
					messageType: controlMessageTypeConnectionClosed,
				}
			} else {
				ctrlMsg = connectionControlMessage{
					messageType: controlMessageTypeError,
					messageBody: fmt.Errorf("conn.Read(): %s", err),
				}
			}
			c.ctrlMsgChan <- ctrlMsg
			return
		}
	}
}

// returns:
//    slice of lines found, stripped of terminating characters
//    number of bytes consumed (so the caller can adjust their write-offset into the buffer)
func findLines(buf []byte) ([][]byte, int) {
	linesFound := false
	var lines [][]byte
	var idx int
	for {
		dataLen, numTermBytes := findLineTerminator(buf[idx:])
		if dataLen == -1 {
			break
		}
		linesFound = true
		// save the line we found
		line := make([]byte, dataLen)
		copy(line, buf[idx:idx+dataLen])
		lines = append(lines, line)
		// advance the index to continue searching for lines
		idx += dataLen + numTermBytes
	}

	// If we chopped out some lines, shift the remaining data in buf up to the
	// front of the buffer to make room for more incoming data.
	if linesFound {
		copy(buf, buf[idx:])
	}

	return lines, idx
}

// Finds lines as terminated according to the RFC 1123 Telnet End-of-Line Convention:
//   https://www.freesoft.org/CIE/RFC/1123/31.htm
// Returns:
//   length of data before line-terminating characters, or -1 if terminator not found
//   # of bytes of line-terminating characters, or -1 if terminator not found
func findLineTerminator(b []byte) (int, int) {
	idx := bytes.Index(b, []byte("\r\n"))
	if idx != -1 {
		return idx, 2
	}
	idx = bytes.Index(b, []byte("\r\x00"))
	if idx != -1 {
		return idx, 2
	}
	idx = bytes.Index(b, []byte("\n"))
	if idx != -1 {
		return idx, 1
	}
	return -1, -1
}

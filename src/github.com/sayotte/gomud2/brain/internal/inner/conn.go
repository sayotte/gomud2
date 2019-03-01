package inner

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sayotte/gomud2/wsapi"
	"time"
)

type Connection struct {
	WSConn *websocket.Conn
}

func (c Connection) sendMessage(msg wsapi.Message) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("json.Marshal(): %s", err)
	}
	return c.sendLowlevelTextMessage(msgBytes)
}

func (c Connection) sendLowlevelTextMessage(body []byte) error {
	err := c.WSConn.WriteMessage(websocket.TextMessage, body)
	if err != nil {
		return fmt.Errorf("conn.WriteMessage(): %s", err)
	}
	return nil
}

func (c Connection) sendCloseMessage(closeCode int, closeText string) {
	payload := websocket.FormatCloseMessage(closeCode, closeText)
	err := c.WSConn.SetWriteDeadline(time.Time{})
	if err != nil {
		panic(err)
	}
	// ignoring error here because we're on the way to shutdown and we've set
	// an unlimited timeout; the only other failures would be writing to an
	// already closed/corrupt connection, in which case we should proceed
	// anyway
	_ = c.WSConn.WriteMessage(websocket.CloseMessage, payload)
}

func (c Connection) getLowlevelMessage() (int, []byte, error) {
	msgType, body, err := c.WSConn.ReadMessage()
	if err != nil {
		return 0, nil, fmt.Errorf("conn.ReadMessage(): %s", err)
	}
	return msgType, body, nil
}

func (c Connection) close() error {
	return c.WSConn.Close()
}

package inner

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/sayotte/gomud2/wsapi"
)

type Connection struct {
	WSConn *websocket.Conn
}

func (c Connection) sendMessage(msg wsapi.Message) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("json.Marshal(): %s", err)
	}
	return c.sendLowlevelMessage(msgBytes)
}

func (c Connection) sendLowlevelMessage(body []byte) error {
	err := c.WSConn.WriteMessage(websocket.TextMessage, body)
	if err != nil {
		return fmt.Errorf("conn.WriteMessage(): %s", err)
	}
	return nil
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

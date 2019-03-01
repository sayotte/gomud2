package intelligence

import "github.com/sayotte/gomud2/wsapi"

type MessageSender interface {
	SendMessage(msg wsapi.Message) error
}

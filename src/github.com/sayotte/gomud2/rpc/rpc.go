package rpc

type Response struct {
	Err   error
	Value interface{}
}

type Request struct {
	ResponseChan chan Response
	Payload      interface{}
}

func NewRequest(payload interface{}) Request {
	return Request{
		ResponseChan: make(chan Response),
		Payload:      payload,
	}
}

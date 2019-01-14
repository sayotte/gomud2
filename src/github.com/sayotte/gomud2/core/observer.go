package core

import "fmt"

type Observer interface {
	SendEvent(e Event)
}

type ObserverList []Observer

func (ol ObserverList) IndexOf(o Observer) (int, error) {
	for i := 0; i < len(ol); i++ {
		if ol[i] == o {
			return i, nil
		}
	}
	return -1, fmt.Errorf("Observer not found in list")
}

func (ol ObserverList) Remove(o Observer) ObserverList {
	idx, err := ol.IndexOf(o)
	if err != nil {
		return ol
	}
	return append(ol[:idx], ol[idx+1:]...)
}

func (ol ObserverList) Copy() ObserverList {
	out := make(ObserverList, len(ol))
	copy(out, ol)
	return out
}
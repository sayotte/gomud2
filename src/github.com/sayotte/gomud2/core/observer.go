package core

import "fmt"

type Observer interface {
	SendEvent(e Event)
	Evict()
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

func (ol ObserverList) Dedupe() ObserverList {
	out := make(ObserverList, len(ol))
	present := make(map[Observer]bool, len(ol))
	for _, o := range ol {
		if !present[o] {
			out = append(out, o)
		}
	}
	return out
}

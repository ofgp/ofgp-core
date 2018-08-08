package util

import (
	"reflect"
	"sync"
)

type Event struct {
	handlers []reflect.Value
	mu       sync.Mutex
}

const (
	UNSUBSCRIBE = true
)

func NewEvent() *Event {
	return &Event{
		handlers: nil,
		mu:       sync.Mutex{},
	}
}

func (e *Event) Size() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.handlers)
}

func (e *Event) Subscribe(h interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.handlers = append(e.handlers, reflect.ValueOf(h))
}

// Emit trigger the handlers that subscribe the interface
func (e *Event) Emit(params ...interface{}) {
	e.mu.Lock()
	defer e.mu.Unlock()

	toRemove := make(map[int]bool)
	if len(e.handlers) > 0 {
		var callArgv []reflect.Value
		for _, p := range params {
			callArgv = append(callArgv, reflect.ValueOf(p))
		}
		for idx, h := range e.handlers {
			rst := h.Call(callArgv)
			if len(rst) == 1 && rst[0].Interface() == interface{}(UNSUBSCRIBE) {
				toRemove[idx] = true
			}
		}
	}

	if len(toRemove) > 0 {
		var afterRemove []reflect.Value
		for idx, h := range e.handlers {
			if !toRemove[idx] {
				afterRemove = append(afterRemove, h)
			}
		}
		e.handlers = afterRemove
	}
}

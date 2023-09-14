package domain

import (
	"enman/internal/log"
	"reflect"
	"sync"
)

type Listener[V any] interface {
	comparable
	HandleEvent(V)
}

type genericEventHandler[L Listener[V], V any] struct {
	listeners map[L]func(values V) bool
	mutex     sync.Mutex
}

func (g *genericEventHandler[L, V]) Register(listener L, filter func(V) bool) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.listeners[listener] = filter
	log.Infof("Registered %s for %s events", reflect.TypeOf(listener).String(), reflect.TypeOf(g.listeners).Elem().In(0).String())
}

func (g *genericEventHandler[L, V]) Trigger(values V) {
	for listener, filter := range g.listeners {
		if filter == nil || filter(values) {
			go listener.HandleEvent(values)
		}
	}
}

func (g *genericEventHandler[L, V]) Deregister(listener L) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	_, ok := g.listeners[listener]
	if ok {
		delete(g.listeners, listener)
		log.Infof("Deregistered %s for %s events", reflect.TypeOf(listener).String(), reflect.TypeOf(g.listeners).Elem().In(0).String())
	}
}

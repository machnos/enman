package events

import (
	"context"
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
	wg        sync.WaitGroup
	ctx       context.Context
	cancel    context.CancelFunc
}

func (g *genericEventHandler[L, V]) Register(listener L, filter func(V) bool) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.listeners[listener] = filter
	log.Infof("Registered %s for %s events", reflect.TypeOf(listener).String(), reflect.TypeOf(g.listeners).Elem().In(0).String())
}

func (g *genericEventHandler[L, V]) Trigger(values V) {
	g.mutex.Lock()
	listeners := make([]L, 0, len(g.listeners))
	filters := make([]func(values V) bool, 0, len(g.listeners))
	for listener, filter := range g.listeners {
		listeners = append(listeners, listener)
		filters = append(filters, filter)
	}
	g.mutex.Unlock()

	for i, listener := range listeners {
		filter := filters[i]
		if filter == nil || filter(values) {
			g.wg.Add(1)
			go func(l L, v V) {
				defer g.wg.Done()
				l.HandleEvent(v)
			}(listener, values)
		}
	}
}

// SetContext sets the context for this event handler (must be called before Trigger is used)
func (g *genericEventHandler[L, V]) SetContext(ctx context.Context) {
	g.ctx = ctx
	g.cancel = nil
	if ctx != nil {
		g.ctx, g.cancel = context.WithCancel(ctx)
	}
}

// WaitForCompletion waits for all pending event handlers to complete or context to be cancelled
func (g *genericEventHandler[L, V]) WaitForCompletion() {
	g.wg.Wait()
}

// Stop cancels context and waits for all handlers to complete
func (g *genericEventHandler[L, V]) Stop() {
	if g.cancel != nil {
		g.cancel()
	}
	g.WaitForCompletion()
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

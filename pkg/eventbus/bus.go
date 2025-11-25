package eventbus

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Handler defines a callback invoked when the client emits an event.
type Handler func(payload []any)

// Bus maintains websocket clients and server-side handlers.
type Bus struct {
	mu sync.RWMutex

	// subscribers maps event names to registered websocket clients.
	subscribers map[string]map[*Client]struct{}

	// handlers maps event names to server-side handlers that listen for
	// events emitted by clients.
	handlers map[string]map[int]Handler

	nextHandlerID int
	upgrader      websocket.Upgrader
}

// New creates a new event bus instance.
func New() *Bus {
	return &Bus{
		subscribers: make(map[string]map[*Client]struct{}),
		handlers:    make(map[string]map[int]Handler),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// ServeWS upgrades the request to a websocket connection and attaches it to the bus.
func (b *Bus) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := b.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := newClient(b, conn)
	go client.readLoop()
	go client.writeLoop()
}

// Emit broadcasts an event to all websocket subscribers.
func (b *Bus) Emit(event string, payload ...any) {
	data, err := json.Marshal(wsMessage{Event: event, Payload: payload})
	if err != nil {
		return
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for client := range b.subscribers[event] {
		client.queue(data)
	}
}

// Subscribe registers a client for an event.
func (b *Bus) Subscribe(event string, client *Client) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.subscribers[event]; !ok {
		b.subscribers[event] = make(map[*Client]struct{})
	}
	b.subscribers[event][client] = struct{}{}
}

// Unsubscribe removes a client from an event.
func (b *Bus) Unsubscribe(event string, client *Client) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs, ok := b.subscribers[event]; ok {
		delete(subs, client)
		if len(subs) == 0 {
			delete(b.subscribers, event)
		}
	}
}

// removeClient removes the client from all subscriptions. Should be invoked when a client disconnects.
func (b *Bus) removeClient(client *Client) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for event, subs := range b.subscribers {
		delete(subs, client)
		if len(subs) == 0 {
			delete(b.subscribers, event)
		}
	}
}

// On registers a server-side handler for events emitted by clients.
func (b *Bus) On(event string, handler Handler) func() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextHandlerID++
	id := b.nextHandlerID

	if _, ok := b.handlers[event]; !ok {
		b.handlers[event] = make(map[int]Handler)
	}
	b.handlers[event][id] = handler

	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		if handlers, ok := b.handlers[event]; ok {
			delete(handlers, id)
			if len(handlers) == 0 {
				delete(b.handlers, event)
			}
		}
	}
}

// emitFromClient dispatches an event emitted by a client to server-side handlers.
func (b *Bus) emitFromClient(event string, payload []any) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, handler := range b.handlers[event] {
		handler(payload)
	}
}

package eventbus

import (
	"time"

	"github.com/gorilla/websocket"
)

type wsMessage struct {
	Action  string `json:"action,omitempty"`
	Event   string `json:"event,omitempty"`
	Payload []any  `json:"payload,omitempty"`
}

type Client struct {
	bus    *Bus
	conn   *websocket.Conn
	send   chan []byte
	closed chan struct{}

	// events keeps track of client subscriptions so we can resubscribe after reconnect.
	events map[string]struct{}
}

func newClient(bus *Bus, conn *websocket.Conn) *Client {
	return &Client{
		bus:    bus,
		conn:   conn,
		send:   make(chan []byte, 64),
		closed: make(chan struct{}),
		events: make(map[string]struct{}),
	}
}

func (c *Client) queue(payload []byte) {
	select {
	case c.send <- payload:
	case <-c.closed:
	}
}

func (c *Client) readLoop() {
	defer c.close()

	c.conn.SetReadLimit(512 * 1024)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg wsMessage
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}

		switch msg.Action {
		case "subscribe":
			c.events[msg.Event] = struct{}{}
			c.bus.Subscribe(msg.Event, c)
		case "unsubscribe":
			delete(c.events, msg.Event)
			c.bus.Unsubscribe(msg.Event, c)
		case "emit":
			c.bus.emitFromClient(msg.Event, msg.Payload)
		case "ping":
			_ = c.conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(5*time.Second))
		}
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
				return
			}
		case <-c.closed:
			return
		}
	}
}

func (c *Client) close() {
	select {
	case <-c.closed:
		return
	default:
		close(c.closed)
	}
	c.bus.removeClient(c)
	c.conn.Close()
	close(c.send)
}

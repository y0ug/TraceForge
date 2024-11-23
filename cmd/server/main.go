package main

import (
	"flag"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var (
	addr   = flag.String("addr", ":8889", "http service address")
	logger = logrus.New()
)

// Connection represents a WebSocket connection.
type Connection struct {
	ws   *websocket.Conn
	send chan []byte
}

// Hub maintains active connections and routes messages.
type Hub struct {
	// Registered clients and agents
	clients map[string]map[*Connection]bool
	agents  map[string]*Connection

	// Inbound messages from agents
	broadcast chan Message

	// Register requests
	register chan Registration

	// Unregister requests
	unregister chan Unregistration

	sync.Mutex
}

type Message struct {
	taskID  string
	message []byte
}

type Registration struct {
	taskID  string
	conn    *Connection
	isAgent bool
}

type Unregistration struct {
	taskID  string
	conn    *Connection
	isAgent bool
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Connection]bool),
		agents:     make(map[string]*Connection),
		broadcast:  make(chan Message),
		register:   make(chan Registration),
		unregister: make(chan Unregistration),
	}
}

func (h *Hub) run() {
	for {
		select {
		case reg := <-h.register:
			h.Lock()
			if reg.isAgent {
				h.agents[reg.taskID] = reg.conn
			} else {
				if h.clients[reg.taskID] == nil {
					h.clients[reg.taskID] = make(map[*Connection]bool)
				}
				h.clients[reg.taskID][reg.conn] = true
			}
			h.Unlock()
			logger.WithFields(logrus.Fields{
				"task_id": reg.taskID,
				"role":    reg.isAgent,
			}).Info("Registered connection")
		case unreg := <-h.unregister:
			h.Lock()
			if unreg.isAgent {
				if conn, ok := h.agents[unreg.taskID]; ok && conn == unreg.conn {
					delete(h.agents, unreg.taskID)
					close(unreg.conn.send)
					logger.WithFields(logrus.Fields{
						"task_id": unreg.taskID,
						"role":    "agent",
					}).Info("Unregistered agent")
				}
			} else {
				if conns, ok := h.clients[unreg.taskID]; ok {
					if _, exists := conns[unreg.conn]; exists {
						delete(conns, unreg.conn)
						close(unreg.conn.send)
						if len(conns) == 0 {
							delete(h.clients, unreg.taskID)
						}
						logger.WithFields(logrus.Fields{
							"task_id": unreg.taskID,
							"role":    "client",
						}).Info("Unregistered client")
					}
				}
			}
			h.Unlock()
		case msg := <-h.broadcast:
			h.Lock()
			if clients, ok := h.clients[msg.taskID]; ok {
				for conn := range clients {
					select {
					case conn.send <- msg.message:
					default:
						close(conn.send)
						delete(clients, conn)
						logger.WithFields(logrus.Fields{
							"task_id": msg.taskID,
						}).Warn("Closed slow client connection")
					}
				}
			}
			h.Unlock()
		}
	}
}

var upgrader = websocket.Upgrader{}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		http.Error(w, "task_id parameter missing", http.StatusBadRequest)
		return
	}

	role := r.URL.Query().Get("role")
	if role != "agent" && role != "client" {
		http.Error(w, "role parameter missing or invalid", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.WithError(err).Error("Upgrade error")
		return
	}

	c := &Connection{
		ws:   conn,
		send: make(chan []byte, 256),
	}

	reg := Registration{
		taskID:  taskID,
		conn:    c,
		isAgent: role == "agent",
	}

	hub.register <- reg

	// Start goroutines to read and write
	go c.writePump()
	go c.readPump(hub, reg)
}

func (c *Connection) readPump(hub *Hub, reg Registration) {
	defer func() {
		hub.unregister <- Unregistration{
			taskID:  reg.taskID,
			conn:    c,
			isAgent: reg.isAgent,
		}
		c.ws.Close()
	}()

	// @TODO do I really need this?
	c.ws.SetReadLimit(512)
	c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.WithFields(logrus.Fields{
					"task_id": reg.taskID,
					"role":    reg.isAgent,
				}).WithError(err).Error("Unexpected read error")
			} else {
				logger.WithFields(logrus.Fields{
					"task_id": reg.taskID,
					"role":    reg.isAgent,
				}).Info("Connection closed")
			}
			break
		}
		if reg.isAgent {
			// Broadcast message to clients
			hub.broadcast <- Message{
				taskID:  reg.taskID,
				message: message,
			}
		}
	}
}

func (c *Connection) writePump() {
	defer c.ws.Close()
	for {
		message, ok := <-c.send
		if !ok {
			// The hub closed the channel.
			c.ws.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
		err := c.ws.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			logger.Println("Write error:", err)
			return
		}
	}
}

func main() {
	flag.Parse()

	logger.SetFormatter(&logrus.TextFormatter{
		DisableColors: false,
		FullTimestamp: true,
	})
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	hub := newHub()
	go hub.run()

	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/", fs)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
	})
	logger.Println("Server started on", *addr)
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		logger.Fatal("ListenAndServe:", err)
	}
}

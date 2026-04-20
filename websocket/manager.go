package websocket

import (
	"encoding/json"
	"log"
	"queueflow/models"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
)

type Client struct {
	Conn   *websocket.Conn
	UserID int
	Role   string
	Send   chan []byte
}

type Manager struct {
	clients    map[int]*Client // userID -> Client
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
}

func NewManager() *Manager {
	return &Manager{
		clients:    make(map[int]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 256),
	}
}

// Run starts the WebSocket manager event loop
func (m *Manager) Run() {
	for {
		select {
		case client := <-m.register:
			m.mu.Lock()

			// 🔄 NEW: Close old connection if exists
			if existingClient, exists := m.clients[client.UserID]; exists {
				log.Printf("Closing existing connection for UserID=%d before registering new one", client.UserID)
				close(existingClient.Send)
				existingClient.Conn.Close()
			}

			m.clients[client.UserID] = client
			m.mu.Unlock()
			log.Printf("Client registered: UserID=%d, Role=%s, Total clients=%d", client.UserID, client.Role, len(m.clients))

		case client := <-m.unregister:
			m.mu.Lock()
			if storedClient, ok := m.clients[client.UserID]; ok {
				// Only close and delete if this is the same client instance
				if storedClient == client {
					delete(m.clients, client.UserID)
					close(client.Send)
					log.Printf("Client unregistered: UserID=%d, Total clients=%d", client.UserID, len(m.clients))
				} else {
					log.Printf("Skipping unregister for UserID=%d (already replaced by new connection)", client.UserID)
				}
			}
			m.mu.Unlock()

		case message := <-m.broadcast:
			m.mu.RLock()
			for _, client := range m.clients {
				select {
				case client.Send <- message:
				default:
					// Client send buffer is full, skip
					log.Printf("Failed to send message to client %d (buffer full)", client.UserID)
				}
			}
			m.mu.RUnlock()
		}
	}
}

// SendToUser sends a message to a specific user
func (m *Manager) SendToUser(userID int, message models.WSMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	m.mu.RLock()
	client, ok := m.clients[userID]
	m.mu.RUnlock()

	if !ok {
		log.Printf("User %d not connected", userID)
		return nil // Not an error if user is not connected
	}

	select {
	case client.Send <- data:
		return nil
	default:
		log.Printf("Failed to send message to user %d (buffer full)", userID)
		return nil
	}
}

// BroadcastToRole sends a message to all clients with a specific role
func (m *Manager) BroadcastToRole(role string, message models.WSMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, client := range m.clients {
		if client.Role == role {
			select {
			case client.Send <- data:
			default:
				log.Printf("Failed to send message to client %d (buffer full)", client.UserID)
			}
		}
	}

	return nil
}

// IsUserConnected checks if a user is connected
func (m *Manager) IsUserConnected(userID int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.clients[userID]
	return ok
}

// HandleClient handles a WebSocket client connection
func (m *Manager) HandleClient(client *Client) {
	// Register the client
	m.register <- client

	// Start write pump in goroutine
	go client.writePump()

	// Read pump runs in current goroutine (blocks until connection closes)
	client.readPump(m)
}

// writePump sends messages from the send channel to the WebSocket connection
func (c *Client) writePump() {
	defer func() {
		c.Conn.Close()
	}()

	for message := range c.Send {
		if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("Error writing to websocket for user %d: %v", c.UserID, err)
			return
		}
	}
}

// readPump reads messages from the WebSocket connection
func (c *Client) readPump(manager *Manager) {
	defer func() {
		manager.unregister <- c
		c.Conn.Close()
	}()

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for user %d: %v", c.UserID, err)
			}
			break
		}

		// Parse incoming message
		var wsMsg models.WSMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			log.Printf("Error parsing message from user %d: %v", c.UserID, err)
			continue
		}

		log.Printf("Received message from user %d: Type=%s", c.UserID, wsMsg.Type)
		// Message handling is done at the service level
	}
}

// UpgradeConnection upgrades HTTP connection to WebSocket
func (m *Manager) UpgradeConnection(c *fiber.Ctx, userID int, role string) error {
	return websocket.New(func(conn *websocket.Conn) {
		client := &Client{
			Conn:   conn,
			UserID: userID,
			Role:   role,
			Send:   make(chan []byte, 256),
		}

		m.HandleClient(client)
	})(c)
}

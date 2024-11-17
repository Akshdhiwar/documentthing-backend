package utils

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Room represents a project room
type Room struct {
	clients map[*websocket.Conn]bool // Connected clients
	mu      sync.Mutex               // Mutex for thread-safe access
}

// Global map to store rooms (keyed by project ID)
var rooms = make(map[string]*Room)
var roomsMu sync.Mutex // Mutex for the rooms map

// WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // Allow all origins (for simplicity)
}

// Get or create a room for a project
func getOrCreateRoom(projectID string) *Room {
	roomsMu.Lock()
	defer roomsMu.Unlock()

	room, exists := rooms[projectID]
	if !exists {
		room = &Room{clients: make(map[*websocket.Conn]bool)}
		rooms[projectID] = room
	}
	return room
}

// Add a client to the room
func (r *Room) addClient(conn *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clients[conn] = true
}

// Remove a client from the room and clean up the room if empty
func (r *Room) removeClient(conn *websocket.Conn, projectID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.clients, conn)

	// If the room is empty, remove it from the global rooms map
	if len(r.clients) == 0 {
		roomsMu.Lock()
		defer roomsMu.Unlock()
		delete(rooms, projectID)
		fmt.Printf("Room for project %s has been removed (no users connected).\n", projectID)
	}
}

// Broadcast a message to all clients in the room (except the sender)
func (r *Room) broadcast(message []byte, sender *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for client := range r.clients {
		if client != sender {
			err := client.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				fmt.Println("Error broadcasting message:", err)
				client.Close()
				delete(r.clients, client)
			}
		}
	}
}

// WebSocket handler
func HandleWebSocket(c *gin.Context) {
	// Get project ID from query parameters
	projectID := c.Param("projectID")

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("Error upgrading to WebSocket:", err)
		return
	}
	defer conn.Close()

	// Add the client to the project room
	room := getOrCreateRoom(projectID)
	room.addClient(conn)
	defer room.removeClient(conn, projectID) // Clean up on disconnect

	fmt.Printf("User connected to project: %s\n", projectID)

	// Listen for messages from the client
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("Error reading message:", err)
			break
		}

		fmt.Printf("Message from project %s: %s\n", projectID, message)

		// Broadcast the message to all clients in the room
		room.broadcast(message, conn)
	}
}

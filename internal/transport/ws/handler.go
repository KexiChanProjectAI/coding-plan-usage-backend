package ws

import (
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Handler handles WebSocket connections.
type Handler struct {
	hub      *Hub
	upgrader websocket.Upgrader
}

// NewHandler creates a new WebSocket handler.
func NewHandler(hub *Hub, upgrader websocket.Upgrader) *Handler {
	return &Handler{
		hub:      hub,
		upgrader: upgrader,
	}
}

// WSHandler returns a gin.HandlerFunc that upgrades HTTP to WebSocket.
func (h *Handler) WSHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			c.Error(err)
			return
		}

		client := NewClient(h.hub, conn)
		h.hub.register <- client

		go client.writePump()
		go client.readPump()
	}
}
package controllers

import (
	"queueflow/websocket"

	"github.com/gofiber/fiber/v2"
)

type WebSocketController struct {
	wsManager *websocket.Manager
}

func NewWebSocketController(wsManager *websocket.Manager) *WebSocketController {
	return &WebSocketController{wsManager: wsManager}
}

func (ctrl *WebSocketController) HandleConnection(c *fiber.Ctx) error {
	// Extract user info from context (set by auth middleware)
	userID := c.Locals("user_id").(int)
	role := c.Locals("role").(string)

	// Upgrade to WebSocket and handle connection
	return ctrl.wsManager.UpgradeConnection(c, userID, role)
}

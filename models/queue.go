package models

import (
	"time"
)

type QueueEntry struct {
	ID          int        `json:"id"`
	UserID      int        `json:"user_id"`
	Username    string     `json:"username,omitempty"`
	Position    int        `json:"position"`
	Status      string     `json:"status"`
	JoinedAt    time.Time  `json:"joined_at"`
	CalledAt    *time.Time `json:"called_at,omitempty"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
	TimeoutAt   *time.Time `json:"timeout_at,omitempty"`
}

type QueueStatus struct {
	Position     int       `json:"position"`
	Status       string    `json:"status"`
	TotalInQueue int       `json:"total_in_queue"`
	EstimatedWait string   `json:"estimated_wait,omitempty"`
	CalledAt     *time.Time `json:"called_at,omitempty"`
	TimeoutAt    *time.Time `json:"timeout_at,omitempty"`
}

type QueueSettings struct {
	ID        int       `json:"id"`
	IsPaused  bool      `json:"is_paused"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WebSocket message types
const (
	WSMessageTypePositionUpdate = "queue:position_update"
	WSMessageTypeYourTurn       = "queue:your_turn"
	WSMessageTypeTimeout        = "queue:timeout"
	WSMessageTypeQueueState     = "admin:queue_state"
	WSMessageTypeConfirm        = "queue:confirm"
	WSMessageTypeError          = "error"
)

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type PositionUpdatePayload struct {
	Position     int    `json:"position"`
	TotalInQueue int    `json:"total_in_queue"`
	Status       string `json:"status"`
}

type YourTurnPayload struct {
	Position     int       `json:"position"`
	TimeoutAt    time.Time `json:"timeout_at"`
	TimeoutInSec int       `json:"timeout_in_seconds"`
}

type TimeoutPayload struct {
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

type QueueStatePayload struct {
	Queue     []QueueEntry `json:"queue"`
	IsPaused  bool         `json:"is_paused"`
	Timestamp time.Time    `json:"timestamp"`
}

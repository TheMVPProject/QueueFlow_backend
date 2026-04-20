package services

import (
	"database/sql"
	"log"
	"queueflow/models"
	"queueflow/repositories"
	"queueflow/websocket"
	"time"
)

type QueueService struct {
	queueRepo *repositories.QueueRepository
	wsManager *websocket.Manager
}

func NewQueueService(queueRepo *repositories.QueueRepository, wsManager *websocket.Manager) *QueueService {
	return &QueueService{
		queueRepo: queueRepo,
		wsManager: wsManager,
	}
}

// JoinQueue adds a user to the queue
func (s *QueueService) JoinQueue(userID int) (*models.QueueEntry, error) {
	entry, err := s.queueRepo.JoinQueue(userID)
	if err != nil {
		return nil, err
	}

	// Broadcast position updates to all users
	go s.BroadcastPositionUpdates()

	return entry, nil
}

// LeaveQueue removes a user from the queue
func (s *QueueService) LeaveQueue(userID int) error {
	err := s.queueRepo.LeaveQueue(userID)
	if err != nil {
		return err
	}

	// Broadcast position updates to all users
	go s.BroadcastPositionUpdates()

	return nil
}

// ConfirmTurn confirms user's turn
func (s *QueueService) ConfirmTurn(userID int) (*models.QueueEntry, error) {
	entry, err := s.queueRepo.ConfirmTurn(userID)
	if err != nil {
		return nil, err
	}

	// Notify user of successful confirmation
	s.wsManager.SendToUser(userID, models.WSMessage{
		Type: "queue:confirmed",
		Payload: map[string]interface{}{
			"message": "Turn confirmed successfully",
		},
	})

	// Call next user automatically after confirmation
	go func() {
		time.Sleep(1 * time.Second) // Small delay for smoother experience
		s.CallNextUser()
	}()

	return entry, nil
}

// GetQueueList retrieves all active queue entries
func (s *QueueService) GetQueueList() ([]models.QueueEntry, error) {
	return s.queueRepo.GetQueueList()
}

// GetUserQueueStatus gets a specific user's queue status
func (s *QueueService) GetUserQueueStatus(userID int) (*models.QueueStatus, error) {
	entry, err := s.queueRepo.GetUserQueueStatus(userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // User not in queue
		}
		return nil, err
	}

	// Count total users in queue
	queue, err := s.queueRepo.GetQueueList()
	if err != nil {
		return nil, err
	}

	status := &models.QueueStatus{
		Position:     entry.Position,
		Status:       entry.Status,
		TotalInQueue: len(queue),
	}

	if entry.CalledAt.Valid {
		status.CalledAt = &entry.CalledAt.Time
	}

	if entry.TimeoutAt.Valid {
		status.TimeoutAt = &entry.TimeoutAt.Time
	}

	return status, nil
}

// CallNextUser calls the next user in queue (admin action)
func (s *QueueService) CallNextUser() (*models.QueueEntry, error) {
	entry, err := s.queueRepo.CallNextUser()
	if err != nil {
		return nil, err
	}

	// Notify the user that it's their turn
	timeoutInSec := int(time.Until(entry.TimeoutAt.Time).Seconds())
	s.wsManager.SendToUser(entry.UserID, models.WSMessage{
		Type: models.WSMessageTypeYourTurn,
		Payload: models.YourTurnPayload{
			Position:     entry.Position,
			TimeoutAt:    entry.TimeoutAt.Time,
			TimeoutInSec: timeoutInSec,
		},
	})

	// Start timeout goroutine
	go s.startTimeoutTimer(entry.ID, entry.UserID, entry.TimeoutAt.Time)

	// Broadcast position updates to all users
	go s.BroadcastPositionUpdates()

	// Broadcast queue state to admins
	go s.BroadcastQueueStateToAdmins()

	return entry, nil
}

// RemoveUser removes a user from the queue (admin action)
func (s *QueueService) RemoveUser(userID int) error {
	err := s.queueRepo.RemoveUserFromQueue(userID)
	if err != nil {
		return err
	}

	// Notify user they were removed
	s.wsManager.SendToUser(userID, models.WSMessage{
		Type: models.WSMessageTypeTimeout,
		Payload: models.TimeoutPayload{
			Message: "You have been removed from the queue",
			Reason:  "removed_by_admin",
		},
	})

	// Broadcast updates
	go s.BroadcastPositionUpdates()
	go s.BroadcastQueueStateToAdmins()

	return nil
}

// PauseQueue pauses the queue
func (s *QueueService) PauseQueue() error {
	err := s.queueRepo.UpdateQueuePauseStatus(true)
	if err != nil {
		return err
	}

	go s.BroadcastQueueStateToAdmins()
	return nil
}

// ResumeQueue resumes the queue
func (s *QueueService) ResumeQueue() error {
	err := s.queueRepo.UpdateQueuePauseStatus(false)
	if err != nil {
		return err
	}

	go s.BroadcastQueueStateToAdmins()
	return nil
}

// BroadcastPositionUpdates sends position updates to all users in queue
func (s *QueueService) BroadcastPositionUpdates() {
	queue, err := s.queueRepo.GetQueueList()
	if err != nil {
		log.Printf("Error getting queue list for broadcast: %v", err)
		return
	}

	totalInQueue := len(queue)

	for _, entry := range queue {
		s.wsManager.SendToUser(entry.UserID, models.WSMessage{
			Type: models.WSMessageTypePositionUpdate,
			Payload: models.PositionUpdatePayload{
				Position:     entry.Position,
				TotalInQueue: totalInQueue,
				Status:       entry.Status,
			},
		})
	}
}

// BroadcastQueueStateToAdmins sends queue state to all admin clients
func (s *QueueService) BroadcastQueueStateToAdmins() {
	queue, err := s.queueRepo.GetQueueList()
	if err != nil {
		log.Printf("Error getting queue list for admin broadcast: %v", err)
		return
	}

	settings, err := s.queueRepo.GetQueueSettings()
	if err != nil {
		log.Printf("Error getting queue settings: %v", err)
		return
	}

	s.wsManager.BroadcastToRole("admin", models.WSMessage{
		Type: models.WSMessageTypeQueueState,
		Payload: models.QueueStatePayload{
			Queue:     queue,
			IsPaused:  settings.IsPaused,
			Timestamp: time.Now(),
		},
	})
}

// startTimeoutTimer starts a goroutine to handle timeout
func (s *QueueService) startTimeoutTimer(entryID, userID int, timeoutAt time.Time) {
	duration := time.Until(timeoutAt)
	if duration <= 0 {
		return // Already timed out
	}

	time.Sleep(duration)

	// Check if user has confirmed
	entry, err := s.queueRepo.GetUserQueueStatus(userID)
	if err != nil || entry.Status != "called" {
		// User already confirmed, removed, or left
		return
	}

	// Timeout the user
	log.Printf("Timing out user %d (entry %d)", userID, entryID)
	err = s.queueRepo.TimeoutUser(entryID)
	if err != nil {
		log.Printf("Error timing out user %d: %v", userID, err)
		return
	}

	// Notify user of timeout
	s.wsManager.SendToUser(userID, models.WSMessage{
		Type: models.WSMessageTypeTimeout,
		Payload: models.TimeoutPayload{
			Message: "Your confirmation time has expired",
			Reason:  "timeout",
		},
	})

	// Call next user automatically
	s.CallNextUser()

	// Broadcast updates
	s.BroadcastPositionUpdates()
	s.BroadcastQueueStateToAdmins()
}

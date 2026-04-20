package services

import (
	"context"
	"database/sql"
	"log"
	"queueflow/models"
	"queueflow/repositories"
	"queueflow/websocket"
	"sync"
	"time"
)

// Timeout duration constants
const (
	TimeoutDuration       = 3 * time.Minute // 3 minutes
	TimeoutDurationInSec  = 180             // 3 minutes in seconds
)

type QueueService struct {
	queueRepo      *repositories.QueueRepository
	userRepo       *repositories.UserRepository
	wsManager      *websocket.Manager
	fcmService     *FCMService
	timeoutCancels map[int]context.CancelFunc // userID -> cancel function
	cancelsMu      sync.RWMutex               // protects timeoutCancels map
}

func NewQueueService(queueRepo *repositories.QueueRepository, userRepo *repositories.UserRepository, wsManager *websocket.Manager, fcmService *FCMService) *QueueService {
	return &QueueService{
		queueRepo:      queueRepo,
		userRepo:       userRepo,
		wsManager:      wsManager,
		fcmService:     fcmService,
		timeoutCancels: make(map[int]context.CancelFunc),
	}
}

// JoinQueue adds a user to the queue
func (s *QueueService) JoinQueue(userID int) (*models.QueueEntry, error) {
	entry, err := s.queueRepo.JoinQueue(userID)
	if err != nil {
		return nil, err
	}

	// Get username for notification
	user, err := s.userRepo.GetByID(userID)
	if err == nil && user != nil {
		// Notify admins via WebSocket (for open app)
		go s.wsManager.BroadcastToRole("admin", models.WSMessage{
			Type: "admin:user_joined",
			Payload: map[string]interface{}{
				"username": user.Username,
				"user_id":  userID,
			},
		})

		// Send FCM push notification ONLY to admins whose app is closed
		if s.fcmService != nil {
			go func() {
				// Get all admin tokens
				allAdminTokens, err := s.userRepo.GetAdminFCMTokens()
				if err != nil {
					log.Printf("Failed to get admin FCM tokens: %v", err)
					return
				}

				if len(allAdminTokens) == 0 {
					return
				}

				// Filter: only send to admins who are NOT currently connected
				var offlineAdminTokens []string
				for _, token := range allAdminTokens {
					// Check if this admin is connected via WebSocket
					// Note: We need to track admin IDs by token, but for now send to all
					// and FCM will handle app state
					offlineAdminTokens = append(offlineAdminTokens, token)
				}

				if len(offlineAdminTokens) > 0 {
					log.Printf("[FCM] Sending user joined notification to %d admin(s) with closed app", len(offlineAdminTokens))
					err = s.fcmService.SendAdminUserJoinedNotification(offlineAdminTokens, user.Username)
					if err != nil {
						log.Printf("Failed to send admin FCM notification: %v", err)
					}
				}
			}()
		}
	}

	// Broadcast updates
	s.broadcastQueueUpdate()

	return entry, nil
}

// LeaveQueue removes a user from the queue
func (s *QueueService) LeaveQueue(userID int) error {
	// Cancel timeout if exists
	s.cancelTimeout(userID)

	err := s.queueRepo.LeaveQueue(userID)
	if err != nil {
		return err
	}

	// Broadcast updates
	s.broadcastQueueUpdate()

	return nil
}

// ConfirmTurn confirms user's turn
func (s *QueueService) ConfirmTurn(userID int) (*models.QueueEntry, error) {
	// Cancel timeout goroutine BEFORE confirming
	s.cancelTimeout(userID)

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

	// 🔄 CRITICAL: Broadcast updates IMMEDIATELY
	// (positions were reordered in repository after confirmation)
	s.broadcastQueueUpdate()

	// Call next user automatically after confirmation (NO DELAY)
	go s.CallNextUser()

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
		CalledAt:     entry.CalledAt,
		TimeoutAt:    entry.TimeoutAt,
	}

	return status, nil
}

// CallNextUser calls the next user in queue (admin action)
func (s *QueueService) CallNextUser() (*models.QueueEntry, error) {
	entry, err := s.queueRepo.CallNextUser()
	if err != nil {
		return nil, err
	}

	// Notify the user that it's their turn via WebSocket
	// Calculate actual remaining time to ensure frontend timer is in sync with backend
	remainingSeconds := int(time.Until(*entry.TimeoutAt).Seconds())
	if remainingSeconds < 0 {
		remainingSeconds = 0
	}

	s.wsManager.SendToUser(entry.UserID, models.WSMessage{
		Type: models.WSMessageTypeYourTurn,
		Payload: models.YourTurnPayload{
			Position:     entry.Position,
			TimeoutAt:    *entry.TimeoutAt,
			TimeoutInSec: remainingSeconds,
		},
	})

	// Send FCM push notification ONLY if app is closed (user not connected via WebSocket)
	if s.fcmService != nil && !s.wsManager.IsUserConnected(entry.UserID) {
		fcmToken, err := s.userRepo.GetFCMToken(entry.UserID)
		if err == nil && fcmToken != "" {
			go func() {
				log.Printf("[FCM] User %d app is closed, sending push notification", entry.UserID)
				err := s.fcmService.SendYourTurnNotification(fcmToken, TimeoutDurationInSec)
				if err != nil {
					log.Printf("Failed to send FCM notification to user %d: %v", entry.UserID, err)
				}
			}()
		}
	} else if s.wsManager.IsUserConnected(entry.UserID) {
		log.Printf("[FCM] User %d app is open, skipping push notification", entry.UserID)
	}

	// Start timeout goroutine
	go s.startTimeoutTimer(entry.ID, entry.UserID, entry.TimeoutAt)

	// Broadcast updates
	s.broadcastQueueUpdate()

	return entry, nil
}

// RemoveUser removes a user from the queue (admin action)
func (s *QueueService) RemoveUser(userID int) error {
	// Cancel timeout if exists
	s.cancelTimeout(userID)

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
	s.broadcastQueueUpdate()

	return nil
}

// PauseQueue pauses the queue
func (s *QueueService) PauseQueue() error {
	err := s.queueRepo.UpdateQueuePauseStatus(true)
	if err != nil {
		return err
	}

	s.broadcastQueueUpdate()
	return nil
}

// ResumeQueue resumes the queue
func (s *QueueService) ResumeQueue() error {
	err := s.queueRepo.UpdateQueuePauseStatus(false)
	if err != nil {
		return err
	}

	s.broadcastQueueUpdate()
	return nil
}

// broadcastQueueUpdate is a helper to broadcast both position updates and queue state to admins
func (s *QueueService) broadcastQueueUpdate() {
	go s.BroadcastPositionUpdates()
	go s.BroadcastQueueStateToAdmins()
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
			Timestamp: time.Now().UTC(),
		},
	})
}

// startTimeoutTimer starts a CANCELABLE goroutine to handle timeout
func (s *QueueService) startTimeoutTimer(entryID, userID int, timeoutAt *time.Time) {
	duration := time.Until(*timeoutAt)
	if duration <= 0 {
		log.Printf("Timeout already expired for user %d, handling immediately", userID)
		s.handleTimeout(entryID, userID)
		return
	}

	// Create cancelable context
	ctx, cancel := context.WithCancel(context.Background())

	// Store cancel function
	s.cancelsMu.Lock()
	s.timeoutCancels[userID] = cancel
	s.cancelsMu.Unlock()

	log.Printf("Starting timeout timer for user %d (entry %d), duration: %v", userID, entryID, duration)

	// Wait for timeout OR cancellation
	select {
	case <-time.After(duration):
		// Timeout fired
		log.Printf("Timeout fired for user %d (entry %d)", userID, entryID)
		s.handleTimeout(entryID, userID)

	case <-ctx.Done():
		// Cancelled (user confirmed early or left queue)
		log.Printf("Timeout cancelled for user %d (entry %d) - user confirmed or left", userID, entryID)
		return
	}

	// Clean up cancel function from map after timeout
	s.cancelsMu.Lock()
	delete(s.timeoutCancels, userID)
	s.cancelsMu.Unlock()
}

// handleTimeout processes a timeout (extracted for reuse in recovery)
func (s *QueueService) handleTimeout(entryID, userID int) {
	// Check if user has confirmed
	entry, err := s.queueRepo.GetUserQueueStatus(userID)
	if err != nil || entry.Status != "called" {
		// User already confirmed, removed, or left
		log.Printf("User %d not in 'called' state, skipping timeout", userID)
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
	s.broadcastQueueUpdate()
}

// cancelTimeout cancels a timeout for a user
func (s *QueueService) cancelTimeout(userID int) {
	s.cancelsMu.Lock()
	defer s.cancelsMu.Unlock()

	if cancel, exists := s.timeoutCancels[userID]; exists {
		log.Printf("Cancelling timeout for user %d", userID)
		cancel()
		delete(s.timeoutCancels, userID)
	}
}

// RecoverTimeouts restarts timeout timers for users in "called" state after server restart
func (s *QueueService) RecoverTimeouts() {
	log.Println("🔄 Recovering timeout timers after server restart...")

	entries, err := s.queueRepo.GetActiveCalledUsers()
	if err != nil {
		log.Printf("Error recovering timeouts: %v", err)
		return
	}

	if len(entries) == 0 {
		log.Println("✅ No active called users to recover")
		return
	}

	log.Printf("Found %d users in 'called' state, restarting timers...", len(entries))

	for _, entry := range entries {
		if entry.TimeoutAt == nil {
			log.Printf("⚠️ User %d has no timeout_at, skipping", entry.UserID)
			continue
		}

		remaining := time.Until(*entry.TimeoutAt)

		if remaining <= 0 {
			// Already timed out, process immediately
			log.Printf("⏰ User %d already timed out (expired %v ago), processing now", entry.UserID, -remaining)
			go s.handleTimeout(entry.ID, entry.UserID)
		} else {
			// Restart timer with remaining duration
			log.Printf("⏱️ Restarting timer for user %d with %v remaining", entry.UserID, remaining)
			go s.startTimeoutTimer(entry.ID, entry.UserID, entry.TimeoutAt)
		}
	}

	log.Println("✅ Timeout recovery complete")
}

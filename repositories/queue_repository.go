package repositories

import (
	"database/sql"
	"fmt"
	"queueflow/models"
	"time"
)

type QueueRepository struct {
	db *sql.DB
}

func NewQueueRepository(db *sql.DB) *QueueRepository {
	return &QueueRepository{db: db}
}

// JoinQueue adds a user to the queue (race-condition safe)
func (r *QueueRepository) JoinQueue(userID int) (*models.QueueEntry, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check if user is already in queue
	var exists bool
	err = tx.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM queue_entries WHERE user_id = $1 AND status IN ('waiting', 'called'))",
		userID,
	).Scan(&exists)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("user already in queue")
	}

	// Get the next position (atomic)
	var nextPosition int
	err = tx.QueryRow("SELECT COALESCE(MAX(position), 0) + 1 FROM queue_entries WHERE status IN ('waiting', 'called')").Scan(&nextPosition)
	if err != nil {
		return nil, err
	}

	// Insert new queue entry
	entry := &models.QueueEntry{}
	err = tx.QueryRow(
		"INSERT INTO queue_entries (user_id, position, status) VALUES ($1, $2, 'waiting') RETURNING id, user_id, position, status, joined_at",
		userID, nextPosition,
	).Scan(&entry.ID, &entry.UserID, &entry.Position, &entry.Status, &entry.JoinedAt)

	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return entry, nil
}

// LeaveQueue removes a user from the queue and reorders positions
func (r *QueueRepository) LeaveQueue(userID int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the user's position before deletion
	var position int
	err = tx.QueryRow(
		"SELECT position FROM queue_entries WHERE user_id = $1 AND status IN ('waiting', 'called') FOR UPDATE",
		userID,
	).Scan(&position)
	if err != nil {
		return fmt.Errorf("user not in queue")
	}

	// Delete the entry
	_, err = tx.Exec("DELETE FROM queue_entries WHERE user_id = $1 AND status IN ('waiting', 'called')", userID)
	if err != nil {
		return err
	}

	// Reorder positions for users after the removed user
	_, err = tx.Exec(
		"UPDATE queue_entries SET position = position - 1 WHERE position > $1 AND status IN ('waiting', 'called')",
		position,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// ConfirmTurn confirms user's turn (race-condition safe)
func (r *QueueRepository) ConfirmTurn(userID int) (*models.QueueEntry, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Lock the row and verify status
	var entry models.QueueEntry
	var calledAt, timeoutAt sql.NullTime
	err = tx.QueryRow(
		`SELECT id, user_id, position, status, joined_at, called_at, timeout_at
		FROM queue_entries
		WHERE user_id = $1 AND status = 'called'
		FOR UPDATE`,
		userID,
	).Scan(&entry.ID, &entry.UserID, &entry.Position, &entry.Status, &entry.JoinedAt, &calledAt, &timeoutAt)

	if calledAt.Valid {
		entry.CalledAt = &calledAt.Time
	}
	if timeoutAt.Valid {
		entry.TimeoutAt = &timeoutAt.Time
	}

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no active turn to confirm")
		}
		return nil, err
	}

	// Check if timeout has passed
	if entry.TimeoutAt != nil && time.Now().After(*entry.TimeoutAt) {
		return nil, fmt.Errorf("confirmation timeout expired")
	}

	// Update status to confirmed
	now := time.Now()
	_, err = tx.Exec(
		"UPDATE queue_entries SET status = 'confirmed', confirmed_at = $1 WHERE id = $2",
		now, entry.ID,
	)
	if err != nil {
		return nil, err
	}

	entry.Status = "confirmed"
	entry.ConfirmedAt = &now

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &entry, nil
}

// GetQueueList retrieves all active queue entries
func (r *QueueRepository) GetQueueList() ([]models.QueueEntry, error) {
	rows, err := r.db.Query(
		`SELECT qe.id, qe.user_id, u.username, qe.position, qe.status, qe.joined_at, qe.called_at, qe.confirmed_at, qe.timeout_at
		FROM queue_entries qe
		JOIN users u ON qe.user_id = u.id
		WHERE qe.status IN ('waiting', 'called')
		ORDER BY qe.position ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Initialize as empty slice to ensure JSON marshals to [] instead of null
	entries := make([]models.QueueEntry, 0)
	for rows.Next() {
		var entry models.QueueEntry
		var calledAt, confirmedAt, timeoutAt sql.NullTime

		err := rows.Scan(
			&entry.ID, &entry.UserID, &entry.Username, &entry.Position,
			&entry.Status, &entry.JoinedAt, &calledAt, &confirmedAt, &timeoutAt,
		)
		if err != nil {
			return nil, err
		}

		// Convert sql.NullTime to *time.Time
		if calledAt.Valid {
			entry.CalledAt = &calledAt.Time
		}
		if confirmedAt.Valid {
			entry.ConfirmedAt = &confirmedAt.Time
		}
		if timeoutAt.Valid {
			entry.TimeoutAt = &timeoutAt.Time
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// GetUserQueueStatus gets a specific user's queue status
func (r *QueueRepository) GetUserQueueStatus(userID int) (*models.QueueEntry, error) {
	entry := &models.QueueEntry{}
	var calledAt, confirmedAt, timeoutAt sql.NullTime

	err := r.db.QueryRow(
		`SELECT qe.id, qe.user_id, qe.position, qe.status, qe.joined_at, qe.called_at, qe.confirmed_at, qe.timeout_at
		FROM queue_entries qe
		WHERE qe.user_id = $1 AND qe.status IN ('waiting', 'called')`,
		userID,
	).Scan(&entry.ID, &entry.UserID, &entry.Position, &entry.Status, &entry.JoinedAt, &calledAt, &confirmedAt, &timeoutAt)

	if err != nil {
		return nil, err
	}

	// Convert sql.NullTime to *time.Time
	if calledAt.Valid {
		entry.CalledAt = &calledAt.Time
	}
	if confirmedAt.Valid {
		entry.ConfirmedAt = &confirmedAt.Time
	}
	if timeoutAt.Valid {
		entry.TimeoutAt = &timeoutAt.Time
	}

	return entry, nil
}

// CallNextUser moves the first waiting user to 'called' status (race-condition safe)
func (r *QueueRepository) CallNextUser() (*models.QueueEntry, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Check if queue is paused
	var isPaused bool
	err = tx.QueryRow("SELECT is_paused FROM queue_settings WHERE id = 1").Scan(&isPaused)
	if err != nil {
		return nil, err
	}
	if isPaused {
		return nil, fmt.Errorf("queue is paused")
	}

	// Get the first waiting user with row lock
	var entry models.QueueEntry
	err = tx.QueryRow(
		`SELECT qe.id, qe.user_id, u.username, qe.position, qe.status, qe.joined_at
		FROM queue_entries qe
		JOIN users u ON qe.user_id = u.id
		WHERE qe.status = 'waiting'
		ORDER BY qe.position ASC
		LIMIT 1
		FOR UPDATE`,
	).Scan(&entry.ID, &entry.UserID, &entry.Username, &entry.Position, &entry.Status, &entry.JoinedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no users waiting in queue")
		}
		return nil, err
	}

	// Update status to 'called' and set timeout (3 minutes)
	now := time.Now()
	timeoutAt := now.Add(3 * time.Minute)
	_, err = tx.Exec(
		"UPDATE queue_entries SET status = 'called', called_at = $1, timeout_at = $2 WHERE id = $3",
		now, timeoutAt, entry.ID,
	)
	if err != nil {
		return nil, err
	}

	entry.Status = "called"
	entry.CalledAt = &now
	entry.TimeoutAt = &timeoutAt

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &entry, nil
}

// RemoveUserFromQueue removes a user by admin (race-condition safe)
func (r *QueueRepository) RemoveUserFromQueue(userID int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Get the user's position
	var position int
	err = tx.QueryRow(
		"SELECT position FROM queue_entries WHERE user_id = $1 AND status IN ('waiting', 'called') FOR UPDATE",
		userID,
	).Scan(&position)
	if err != nil {
		return fmt.Errorf("user not in queue")
	}

	// Update status to 'removed'
	_, err = tx.Exec("UPDATE queue_entries SET status = 'removed' WHERE user_id = $1", userID)
	if err != nil {
		return err
	}

	// Reorder positions
	_, err = tx.Exec(
		"UPDATE queue_entries SET position = position - 1 WHERE position > $1 AND status IN ('waiting', 'called')",
		position,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// TimeoutUser marks a user as timed out (race-condition safe)
func (r *QueueRepository) TimeoutUser(entryID int) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Lock and verify the entry is still in 'called' status
	var position int
	var status string
	err = tx.QueryRow(
		"SELECT position, status FROM queue_entries WHERE id = $1 FOR UPDATE",
		entryID,
	).Scan(&position, &status)

	if err != nil {
		return err
	}

	// Only timeout if still in 'called' status
	if status != "called" {
		return nil // Already confirmed or removed, no action needed
	}

	// Update status to 'timeout'
	_, err = tx.Exec("UPDATE queue_entries SET status = 'timeout' WHERE id = $1", entryID)
	if err != nil {
		return err
	}

	// Reorder positions for remaining users
	_, err = tx.Exec(
		"UPDATE queue_entries SET position = position - 1 WHERE position > $1 AND status IN ('waiting', 'called')",
		position,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// GetQueueSettings retrieves queue settings
func (r *QueueRepository) GetQueueSettings() (*models.QueueSettings, error) {
	settings := &models.QueueSettings{}
	err := r.db.QueryRow(
		"SELECT id, is_paused, updated_at FROM queue_settings WHERE id = 1",
	).Scan(&settings.ID, &settings.IsPaused, &settings.UpdatedAt)
	return settings, err
}

// UpdateQueuePauseStatus updates queue pause status
func (r *QueueRepository) UpdateQueuePauseStatus(isPaused bool) error {
	_, err := r.db.Exec(
		"UPDATE queue_settings SET is_paused = $1, updated_at = $2 WHERE id = 1",
		isPaused, time.Now(),
	)
	return err
}

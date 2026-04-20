package repositories

import (
	"database/sql"
	"log"
	"queueflow/models"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByUsername(username string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(
		"SELECT id, username, password_hash, role, created_at FROM users WHERE username = $1",
		username,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) GetByID(userID int) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(
		"SELECT id, username, password_hash, role, created_at FROM users WHERE id = $1",
		userID,
	).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) UpdateFCMToken(userID int, fcmToken string) error {
	if fcmToken == "" {
		log.Printf("[UserRepo] Clearing FCM token for user ID: %d", userID)
	} else {
		log.Printf("[UserRepo] Updating FCM token for user ID: %d (token: %s...)", userID, fcmToken[:20])
	}

	_, err := r.db.Exec(
		"UPDATE users SET fcm_token = $1 WHERE id = $2",
		fcmToken, userID,
	)

	if err != nil {
		log.Printf("[UserRepo] Failed to update FCM token for user ID %d: %v", userID, err)
		return err
	}

	log.Printf("[UserRepo] ✅ FCM token updated successfully for user ID: %d", userID)
	return nil
}

func (r *UserRepository) GetFCMToken(userID int) (string, error) {
	var fcmToken sql.NullString
	err := r.db.QueryRow(
		"SELECT fcm_token FROM users WHERE id = $1",
		userID,
	).Scan(&fcmToken)

	if err != nil {
		return "", err
	}

	if fcmToken.Valid {
		return fcmToken.String, nil
	}

	return "", nil
}

func (r *UserRepository) Create(username, email, passwordHash, role string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(
		"INSERT INTO users (username, email, password_hash, role) VALUES ($1, $2, $3, $4) RETURNING id, username, email, role, created_at",
		username, email, passwordHash, role,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	user.PasswordHash = passwordHash
	return user, nil
}

func (r *UserRepository) GetByEmail(email string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(
		"SELECT id, username, email, password_hash, role, created_at FROM users WHERE email = $1",
		email,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt)

	if err != nil {
		return nil, err
	}

	return user, nil
}

// GetAdminFCMTokens returns all FCM tokens for admin users
func (r *UserRepository) GetAdminFCMTokens() ([]string, error) {
	rows, err := r.db.Query(
		"SELECT fcm_token FROM users WHERE role = 'admin' AND fcm_token IS NOT NULL AND fcm_token != ''",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			log.Printf("[UserRepo] Error scanning admin FCM token: %v", err)
			continue
		}
		tokens = append(tokens, token)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	log.Printf("[UserRepo] Found %d admin FCM tokens", len(tokens))
	return tokens, nil
}

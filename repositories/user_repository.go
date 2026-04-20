package repositories

import (
	"database/sql"
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

package config

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func ConnectDatabase(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Set timezone to UTC for consistency
	_, err = db.Exec("SET TIME ZONE 'UTC'")
	if err != nil {
		log.Printf("Warning: Could not set timezone to UTC: %v", err)
	}

	log.Println("Database connection established successfully")
	return db, nil
}

func RunMigrations(db *sql.DB) error {
	log.Println("Running database migrations...")

	migrations := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			role VARCHAR(50) NOT NULL DEFAULT 'user',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,

		// Queue entries table
		`CREATE TABLE IF NOT EXISTS queue_entries (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			position INTEGER NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'waiting',
			joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			called_at TIMESTAMP,
			confirmed_at TIMESTAMP,
			timeout_at TIMESTAMP,
			CONSTRAINT valid_status CHECK (status IN ('waiting', 'called', 'confirmed', 'timeout', 'removed'))
		)`,

		// Index for faster queue queries
		`CREATE INDEX IF NOT EXISTS idx_queue_status ON queue_entries(status)`,
		`CREATE INDEX IF NOT EXISTS idx_queue_position ON queue_entries(position)`,
		`CREATE INDEX IF NOT EXISTS idx_queue_user_id ON queue_entries(user_id)`,

		// Queue settings table (for pause/resume)
		`CREATE TABLE IF NOT EXISTS queue_settings (
			id INTEGER PRIMARY KEY DEFAULT 1,
			is_paused BOOLEAN DEFAULT FALSE,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT single_row CHECK (id = 1)
		)`,

		// Insert default queue settings
		`INSERT INTO queue_settings (id, is_paused) VALUES (1, FALSE) ON CONFLICT (id) DO NOTHING`,

		// Add FCM token column to users table (for push notifications)
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS fcm_token VARCHAR(255)`,

		// Create index for faster FCM token lookups
		`CREATE INDEX IF NOT EXISTS idx_users_fcm_token ON users(fcm_token)`,

		// Add email column to users table
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS email VARCHAR(255) UNIQUE`,

		// Create index for faster email lookups
		`CREATE INDEX IF NOT EXISTS idx_users_email ON users(email)`,
	}

	for _, migration := range migrations {
		if _, err := db.Exec(migration); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	log.Println("Database migrations completed successfully")
	return nil
}

// CreateDefaultUsers creates default admin and test users
func CreateDefaultUsers(db *sql.DB, passwordHash string) error {
	// Check if users already exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		log.Println("Users already exist, skipping default user creation")
		return nil
	}

	log.Println("Creating default users...")

	// Create admin user (username: admin, password: admin123)
	_, err = db.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3)",
		"admin", passwordHash, "admin",
	)
	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	// Create test user (username: user1, password: user123)
	_, err = db.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES ($1, $2, $3)",
		"user1", passwordHash, "user",
	)
	if err != nil {
		return fmt.Errorf("failed to create test user: %w", err)
	}

	log.Println("Default users created successfully (admin/admin123, user1/user123)")
	return nil
}

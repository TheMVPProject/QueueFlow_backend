-- QueueFlow Database Initialization Script
-- This script creates all necessary tables, indexes, and default data

-- Set timezone to UTC
SET TIME ZONE 'UTC';

-- Drop existing tables if needed (uncomment for fresh start)
-- DROP TABLE IF EXISTS queue_entries CASCADE;
-- DROP TABLE IF EXISTS queue_settings CASCADE;
-- DROP TABLE IF EXISTS users CASCADE;

-- ==============================================================================
-- USERS TABLE
-- ==============================================================================
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL DEFAULT 'user',
    fcm_token VARCHAR(255),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for users table
CREATE INDEX IF NOT EXISTS idx_users_fcm_token ON users(fcm_token);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- ==============================================================================
-- QUEUE ENTRIES TABLE
-- ==============================================================================
CREATE TABLE IF NOT EXISTS queue_entries (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    position INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'waiting',
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    called_at TIMESTAMP,
    confirmed_at TIMESTAMP,
    timeout_at TIMESTAMP,
    CONSTRAINT valid_status CHECK (status IN ('waiting', 'called', 'confirmed', 'timeout', 'removed'))
);

-- Create indexes for queue_entries table
CREATE INDEX IF NOT EXISTS idx_queue_status ON queue_entries(status);
CREATE INDEX IF NOT EXISTS idx_queue_position ON queue_entries(position);
CREATE INDEX IF NOT EXISTS idx_queue_user_id ON queue_entries(user_id);

-- ==============================================================================
-- QUEUE SETTINGS TABLE
-- ==============================================================================
CREATE TABLE IF NOT EXISTS queue_settings (
    id INTEGER PRIMARY KEY DEFAULT 1,
    is_paused BOOLEAN DEFAULT FALSE,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT single_row CHECK (id = 1)
);

-- Insert default queue settings (queue is not paused by default)
INSERT INTO queue_settings (id, is_paused)
VALUES (1, FALSE)
ON CONFLICT (id) DO NOTHING;

-- ==============================================================================
-- DEFAULT USERS (password: password123 for both)
-- ==============================================================================
-- Note: The password hash below is for "password123"
-- Generated using bcrypt with cost 10

-- Admin user (username: admin, password: password123)
INSERT INTO users (username, email, password_hash, role)
VALUES (
    'admin',
    'admin@queueflow.com',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', -- password123
    'admin'
) ON CONFLICT (username) DO NOTHING;


-- ==============================================================================
-- VERIFICATION QUERIES
-- ==============================================================================
-- Uncomment below to verify the setup

-- SELECT 'Users created:' AS info;
-- SELECT id, username, email, role, created_at FROM users;

-- SELECT 'Queue settings:' AS info;
-- SELECT * FROM queue_settings;

-- SELECT 'Database initialization complete!' AS status;

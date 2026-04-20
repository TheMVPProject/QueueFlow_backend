-- Add FCM token column to users table
ALTER TABLE users ADD COLUMN fcm_token VARCHAR(255);

-- Create index for faster lookups
CREATE INDEX idx_users_fcm_token ON users(fcm_token);

-- Reset users for QueueFlow
DELETE FROM queue_entries;
DELETE FROM users;

-- Note: Passwords will be recreated on next server start
-- Both admin and user1 will have password: password123

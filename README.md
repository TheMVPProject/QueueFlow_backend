# QueueFlow Backend

Go-based backend server for the QueueFlow virtual queue system.

## Quick Start

```bash
# Install dependencies
go mod download

# Set up environment
cp .env.example .env

# Run server
go run main.go
```

Server starts at `http://localhost:8080`

## Environment Variables

```env
DATABASE_URL=postgres://username:password@localhost:5432/queueflow?sslmode=disable
JWT_SECRET=your-secret-key
PORT=8080
```

## Default Accounts

- **Admin**: `admin` / `password123`
- **User**: `user1` / `password123`

## API Endpoints

- `POST /auth/login` - Authenticate user
- `POST /queue/join` - Join queue
- `POST /queue/leave` - Leave queue
- `POST /queue/confirm` - Confirm turn
- `GET /queue/status` - Get queue status
- `GET /queue/list` - List all queue entries
- `POST /admin/next` - Call next user (admin)
- `POST /admin/remove-user/:id` - Remove user (admin)
- `POST /admin/pause` - Pause queue (admin)
- `POST /admin/resume` - Resume queue (admin)
- `GET /ws` - WebSocket connection

## Project Structure

```
QueueFlow_backend/
├── config/          # Configuration and database setup
├── controllers/     # HTTP handlers
├── middleware/      # Authentication middleware
├── models/          # Data models
├── repositories/    # Database access layer
├── services/        # Business logic
├── websocket/       # WebSocket manager
└── main.go          # Application entry point
```

## Railway Deployment

1. Create Railway project: `railway init`
2. Add PostgreSQL database
3. Set environment variables
4. Deploy: `railway up`

## Architecture Highlights

- **Race Condition Prevention**: PostgreSQL transactions with `SELECT FOR UPDATE`
- **Server-Side Timeouts**: Goroutines with database-backed state
- **WebSocket Management**: Goroutine-safe connection registry
- **Clean Architecture**: Separated concerns (controllers, services, repositories)

See main [README.md](../README.md) for comprehensive documentation.

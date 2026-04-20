# QueueFlow Backend (Go + PostgreSQL + WebSocket)

## 🚀 Overview

QueueFlow Backend is a real-time virtual queue management system built using Go. It allows users to join a queue remotely, receive real-time updates, and confirm their turn within a 3-minute window.

This backend is **server-authoritative**, ensuring correct queue order, timeout handling, and race condition safety.

---

## 🧱 Tech Stack

* Go (Fiber)
* PostgreSQL
* WebSocket (real-time updates)
* Firebase Cloud Messaging (FCM)
* JWT Authentication

---

## ⚙️ Features

* Join / Leave Queue
* Real-time queue updates via WebSocket
* Admin controls (call next, remove user, pause/resume)
* 3-minute timeout logic (server-side)
* FCM push notifications
* Race condition prevention using DB transactions

---

## 🔗 Live Deployment

Backend URL:

```
https://queueflowbackend-production.up.railway.app
```

WebSocket:

```
wss://queueflowbackend-production.up.railway.app/ws
```

---

## 📡 API Endpoints

### Auth

* `POST /auth/register`
* `POST /auth/login`

### Queue

* `POST /queue/join`
* `POST /queue/leave`
* `POST /queue/confirm`
* `GET /queue/status`

### Admin

* `POST /admin/call-next`
* `POST /admin/remove/:id`
* `POST /admin/pause`
* `POST /admin/resume`

---

## 🔌 WebSocket Events

| Event                 | Direction       | Description      |
| --------------------- | --------------- | ---------------- |
| queue:position_update | Server → Client | Position changed |
| queue:your_turn       | Server → Client | User's turn      |
| queue:timeout         | Server → Client | User timed out   |
| admin:queue_state     | Server → Admin  | Full queue       |

---

## 🔐 Authentication

* JWT-based authentication
* Roles:

  * `user`
  * `admin`

---

## ⚡ Setup (Local)

```bash
git clone <repo>
go mod download
```

### Environment Variables

```
DATABASE_URL=postgres://...
JWT_SECRET=your_secret
PORT=8080
FIREBASE_CREDENTIALS_BASE64=your_base64_json
```

Run:

```bash
go run main.go
```

---

## 🧠 Architecture

* Controllers → HTTP layer
* Services → Business logic
* Repository → DB access
* WebSocket Manager → real-time events
* REST API handles state mutations (join, leave, confirm)
* WebSocket handles real-time state propagation

---

## 🔄 WebSocket Behavior

- WebSocket connection is established after authentication
- Clients automatically reconnect on disconnection
- Server broadcasts queue updates to all connected clients
- On reconnect, clients rely on latest broadcast or API sync to restore state

## 🧩 Queue State & Scalability Considerations

### Queue State Storage

- Queue state is stored in **PostgreSQL (persistent storage)**
- Each queue entry is stored with:
  - position
  - status (waiting, called, confirmed, timed_out)
  - timestamps (joined_at, timeout_at, confirmed_at)

- Database is the **single source of truth**
- WebSocket is used only for **real-time broadcasting**

---

### Concurrency & Race Condition Handling

- All critical operations use **database transactions**
- `SELECT ... FOR UPDATE` is used when calling next user
- Prevents:
  - duplicate slot assignment
  - double confirmation
  - inconsistent queue ordering

---

### Timeout Handling

- Timeout (3 minutes) is handled **server-side**
- Each "called" user is assigned a `timeout_at` timestamp stored in the database
- Server-side logic enforces timeout based on this value
- If user does not confirm:
  - status → `timed_out`
  - removed from queue
  - next user can be called

---

### Scalability (500+ Users)

- PostgreSQL ensures safe concurrent access
- Indexed queries optimize performance
- WebSocket supports real-time updates for all connected clients

This setup can handle **500+ concurrent users on a single instance**

---

### Limitations

- WebSocket connections are managed **in-memory (single instance)**
- Horizontal scaling is not yet supported
- No distributed event system (e.g., Redis Pub/Sub)

---

### Server Restart Behavior

- Database persists queue state
- On restart:
  - Active WebSocket connections are lost
  - Backend attempts to **recover active timeouts**
- Without recovery logic, "called" users could get stuck

---

### Future Improvements

- Redis Pub/Sub for distributed WebSocket events
- Background job queue for durable timeout handling
- Multi-instance deployment support
- Load balancing with shared state layer

## ⚠️ Known Limitations

- WebSocket layer is single-instance and not horizontally scalable
- No distributed messaging system (e.g., Redis Pub/Sub)
- Timeout handling relies on in-memory goroutines (partially mitigated with recovery logic)
- FCM depends on valid device tokens and correct client setup
- Cold-start notification navigation can be improved further
- Multi-device session handling is not fully enforced (same user on multiple devices)

---

## 📌 What I Would Improve With More Time

* Distributed WebSocket scaling (Redis Pub/Sub)
* Better reconnection state sync
* Advanced admin analytics
* Improved background notification reliability

---

## 🧪 Testing Checklist

* Join queue ✔
* Real-time position updates ✔
* Admin call next ✔
* Timeout after 3 minutes ✔
* Confirm presence ✔
* WebSocket reconnect ✔

---

## 📖 Notes

This implementation prioritizes:

* Correct queue logic
* Real-time reliability
* Server-side control over critical operations

---

## 👨‍💻 Author

mohidsk
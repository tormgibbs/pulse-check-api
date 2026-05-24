# Pulse Check API

A Dead Man's Switch API for CritMon Servers Inc. Devices register a monitor with a timeout. If the device fails to send a heartbeat before the timer expires, the system fires an alert. Built in Go with Postgres persistence.

---

## Architecture

### Sequence Diagram

![Sequence Diagram](docs/sequence-diagram.png)

### State Flowchart

![State Flowchart](docs/state-flowchart.png)

### How It Works

Each monitor is a countdown timer backed by a Postgres row. When a device registers, the server persists the monitor and spawns a goroutine that counts down from the configured timeout. If a heartbeat arrives before the timer expires, the goroutine resets the countdown and updates `last_heartbeat_at` in the database. If the timer fires without a heartbeat, the monitor transitions to `down` and an alert is logged.

On server restart, all active and recovering monitors are rebuilt from the database. The remaining time is computed from `last_heartbeat_at` rather than stored explicitly, so no countdown state is lost across restarts. Monitors whose deadline already passed during downtime are transitioned to `down` synchronously before the server accepts any connections.

Paused and down monitors have no running goroutine. A heartbeat on a paused monitor spawns a new goroutine and resumes the countdown. A heartbeat on a down monitor starts the recovery process.

### Live URL

https://pulse-check-api-9h3ev.ondigitalocean.app

---

## Stack

- **Language:** Go
- **Router:** chi
- **Database:** Postgres via pgx/v5
- **Migrations:** golang-migrate
- **Logging:** log/slog
- **Deployment:** Digital Ocean App Platform

---

## Setup

### Prerequisites

- Go 1.25+
- Postgres 18
- [golang-migrate CLI](https://github.com/golang-migrate/migrate)

### Local Development

```bash
git clone https://github.com/tormgibbs/pulse-check-api
cd pulse-check-api
cp .env.example .env
# fill in your DATABASE_URL in .env
make migrate-up
make run
```

### Docker Compose

If you prefer not to install Postgres locally:

```bash
docker compose up
```

The app and database start together. Migrations run automatically on startup.

---

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `DATABASE_URL` | Yes | - | Postgres connection string |
| `PORT` | No | `8080` | Port the server listens on |

---

## API

### Endpoints

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Health check |
| `POST` | `/monitors` | Register a monitor |
| `POST` | `/monitors/:id/heartbeat` | Send a heartbeat |
| `POST` | `/monitors/:id/pause` | Pause a monitor |
| `GET` | `/monitors/:id` | Get monitor state |
| `GET` | `/monitors` | List all monitors |
| `DELETE` | `/monitors/:id` | Delete a monitor |

### Register a Monitor

```
POST /monitors
```

```json
{
  "id": "device-123",
  "timeout": 60,
  "failure_threshold": 2,
  "recovery_threshold": 3,
  "alert_on_recovery": true
}
```

| Field | Type | Required | Default | Constraints |
|---|---|---|---|---|
| `id` | string | Yes | - | URL-safe chars, max 64 |
| `timeout` | int | Yes | - | 10 to 86400 seconds |
| `failure_threshold` | int | No | `1` | Min 1 |
| `recovery_threshold` | int | No | `3` | Min 1 |
| `recovery_window` | int | No | `recovery_threshold * timeout` | Seconds |
| `alert_on_recovery` | bool | No | `true` | - |

Response `201 Created`:

```json
{
  "id": "device-123",
  "message": "monitor created"
}
```

### Heartbeat

```
POST /monitors/:id/heartbeat
```

Response `200 OK`:

```json
{
  "message": "heartbeat received"
}
```

### Pause

```
POST /monitors/:id/pause
```

Response `200 OK`:

```json
{
  "message": "monitor paused"
}
```

Calling heartbeat on a paused monitor resumes it.

### Get Monitor

```
GET /monitors/:id
```

Response `200 OK`:

```json
{
  "id": "device-123",
  "status": "active",
  "timeout_seconds": 60,
  "failure_count": 0,
  "consecutive_hb": 0,
  "last_heartbeat_at": "2026-05-21T09:00:00Z"
}
```

### List Monitors

Response `200 OK`:

```
GET /monitors
```

```json
{
  "monitors": [
    {
      "id": "device-123",
      "timeout_seconds": 60,
      "failure_threshold": 2,
      "failure_count": 0,
      "recovery_threshold": 3,
      "recovery_window": 180,
      "consecutive_hb": 0,
      "alert_on_recovery": true,
      "status": "active",
      "last_heartbeat_at": "2026-05-21T10:53:28.844277Z",
      "recovery_deadline": null,
      "alerted_at": null,
      "created_at": "2026-05-21T10:53:28.842019Z",
      "updated_at": "2026-05-21T10:53:28.843640Z"
    }
  ],
  "total": 1
}
```

### Delete Monitor

```
DELETE /monitors/:id
```

Response `204 No Content`; no response body.


### Error Responses

All errors use this shape:

```json
{
  "error": "monitor not found",
  "code": "MONITOR_NOT_FOUND"
}
```

| Code | HTTP Status |
|---|---|
| `MONITOR_NOT_FOUND` | 404 |
| `DUPLICATE_MONITOR` | 409 |
| `INVALID_TRANSITION` | 409 |
| `VALIDATION_ERROR` | 422 |
| `INTERNAL_ERROR` | 500 |

---

## Developer's Choice: Circuit Breaker Recovery State Machine

The base spec treats a timeout as a binary event: the device is either alive or dead. In practice, devices in poor connectivity environments like solar farms and weather stations can miss a heartbeat due to a temporary network issue, not an actual failure. Alerting immediately on the first miss creates noise and erodes trust in the monitoring system.

This is the same problem AWS Route 53 health checks and CloudWatch alarms solve with failure thresholds and recovery thresholds. A resource is not marked unhealthy on the first failed check, and it is not marked healthy again on the first successful one. The system requires consecutive successes or failures before changing state. The same principle applies here.

The circuit breaker pattern adds three things:

**failure_threshold:** the monitor tolerates up to N consecutive missed heartbeats before transitioning to `down`. Each miss increments `failure_count`. A successful heartbeat resets it to zero.

**Recovery state:** when a down monitor receives a heartbeat, it does not immediately return to `active`. It enters `recovering` and starts a recovery window timer. The device must send `recovery_threshold` consecutive heartbeats within the `recovery_window` to prove it has stabilised. If it misses a heartbeat during recovery, it returns to `down` immediately.

**alert_on_recovery:** when a monitor successfully completes recovery and returns to `active`, the system fires a recovery alert so the on-call engineer knows the device is back online without checking manually.

State transitions:

```
active     + missed heartbeat (count < threshold)  -> active (increment failure_count)
active     + missed heartbeat (count >= threshold) -> down (fire alert)
down       + heartbeat                             -> recovering (start recovery window)
recovering + heartbeat (consecutive < threshold)   -> recovering
recovering + heartbeat (consecutive >= threshold)  -> active (fire recovery alert)
recovering + window expired                        -> down (fire alert)
active     + pause                                 -> paused
paused     + heartbeat                             -> active
```

This gives CritMon a system that distinguishes between a device with a flaky connection and a device that is genuinely offline, which is the actual problem they need to solve.

---

## Running Tests

```bash
# unit tests
go test ./internal/handler/... -v

# integration tests (requires Docker)
go test ./internal/store/... -v -timeout 120s

# watcher unit tests
go test ./internal/watcher/... -v
```

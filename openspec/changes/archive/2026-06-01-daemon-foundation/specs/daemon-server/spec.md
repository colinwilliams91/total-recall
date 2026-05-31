## ADDED Requirements

### Requirement: Daemon binds to localhost:7331 on startup
The daemon SHALL bind an HTTP server to `localhost:7331` when `total-recall serve` is invoked. If the port is already in use, the daemon SHALL exit with a clear error message identifying the port conflict and suggesting the user check for an existing daemon process.

#### Scenario: Successful server startup
- **WHEN** the user runs `total-recall serve` and port 7331 is available
- **THEN** the daemon starts, logs a startup message to stdout, and begins accepting requests

#### Scenario: Port already in use
- **WHEN** the user runs `total-recall serve` and port 7331 is already bound
- **THEN** the daemon exits with a non-zero code and prints a message indicating the port conflict

---

### Requirement: Daemon registers /hooks/* and /mcp/* route groups
The daemon SHALL register two logical route groups on startup: `/hooks/*` for hook event ingestion and `/mcp/*` for MCP protocol messages. Routes within each group SHALL be handled by their respective subsystems.

#### Scenario: Hook route responds to POST
- **WHEN** a POST request is made to `/hooks/pre-commit`, `/hooks/commit-msg`, or `/hooks/pre-push`
- **THEN** the daemon returns HTTP 202 Accepted with a JSON body `{ "status": "received" }`

#### Scenario: Unknown route returns 404
- **WHEN** a request is made to an unregistered path
- **THEN** the daemon returns HTTP 404 with a JSON error body

---

### Requirement: Daemon exposes a health endpoint
The daemon SHALL expose `GET /health` that returns `HTTP 200` with a JSON body containing at minimum `{ "status": "ok" }`. This endpoint is used by `total-recall status` to determine if the daemon is alive.

#### Scenario: Health check on running daemon
- **WHEN** a GET request is made to `/health`
- **THEN** the daemon responds with HTTP 200 and `{ "status": "ok" }`

---

### Requirement: Daemon loads user config on startup
The daemon SHALL invoke `config.EnsureUserConfig` (respecting the `--quiet` flag) before binding the port. If the user config cannot be loaded or created, the daemon SHALL exit with a descriptive error.

#### Scenario: Missing user config
- **WHEN** `total-recall serve` is run and `~/.tr/config.yaml` does not exist
- **THEN** `EnsureUserConfig` auto-creates it and prints an advisory (unless `--quiet`), then the daemon proceeds to start

#### Scenario: Corrupt user config
- **WHEN** `total-recall serve` is run and `~/.tr/config.yaml` contains invalid YAML
- **THEN** the daemon exits with a non-zero code and a descriptive parse error

---

### Requirement: Daemon shuts down gracefully on SIGTERM and SIGINT
The daemon SHALL handle OS signals SIGTERM and SIGINT by initiating a graceful HTTP server shutdown with a reasonable drain timeout (5 seconds), then exiting with code 0.

#### Scenario: User presses Ctrl+C
- **WHEN** the daemon is running and the user sends SIGINT (e.g., Ctrl+C)
- **THEN** the daemon completes in-flight requests (up to 5 seconds), then exits cleanly with code 0

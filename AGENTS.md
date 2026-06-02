# KOSYNC - Lightweight KOReader Position Sync Server

KOSYNC is a lightweight server designed to facilitate synchronization of ebooks across all of a user's KOReader devices.  KOReader essentially treats the server as a remote key-value store, so the main task is building a simple CRUD (Create, Read, Update, Delete) API.  There is an official KOReader Sync Server that ships with KOReader, but it has a number of drawbacks.  The intended audience for KOSYNC is home labbers, who may be trying to self-host on very resource constrained hardware.  The code and the README needs to be very well documented to assist novices understand how it works and how to deploy  and troubleshoot it.

## Cross-Project Uniformity

KOSYNC is maintained alongside KOPDS with a maximum-uniformity goal. Functions that perform the same job in both repositories should use the same names and identical code wherever practical. See `../uniformity-plan.md` for the current inventory and boundaries. Keep CLI user management, password helpers, logger construction, config path resolution, SQLite opening, and storage-cap helper flow aligned unless a documented project-specific domain difference requires divergence.

## Project Overview

- **Core Technologies:**
  - **Language:** Go (Golang) for a single-binary, low-memory footprint. Version: 1.22.
  - **Database:** Pure Go SQLite (`modernc.org/sqlite`) for local indexing and multi-user support.
  - **Web Framework:** Lightweight routing with Go's standard HTTP router (`net/http.ServeMux`).
  - **Security:** Bcrypt (`golang.org/x/crypto/bcrypt`) for password hashing.
- **Architecture:**
  - **Clean Architecture:** Separation of domain logic, databases, UI, frameworks. (Note: For simplicity and novice-friendliness, consider keeping database logic in a single file within the storage layer).
  - **Deployment:** Ships as a standalone, single-executable binary for bare-metal execution, or in a Docker container.
- **Logging:** Use Go's standard `log/slog` package for structured logs, preserving request-scoped fields, shutdown events, and storage maintenance diagnostics across both KOSYNC and KOPDS.

## Logging Strategy

KOSYNC uses the standard library `log/slog` for structured logging across the entire application.
- **Uniformity:** Identical logging patterns and field names are used in both KOPDS and KOSYNC.
- **Request Context:** Every HTTP request is assigned a unique `request_id`. A request-scoped logger is stored in the context and should be retrieved via `api.GetLogger(ctx)`.
- **Layers:**
    - **Middleware:** Outermost `LoggingMiddleware` logs request completion (INFO for 2xx/3xx, WARN for 4xx, ERROR for 5xx) with `duration` and `status_code`.
    - **Handlers:** Log high-level business events (e.g., "progress retrieved") at INFO level using the request-scoped logger.
    - **Storage:** Logs granular diagnostic data (e.g., SQL queries, storage cap checks) at DEBUG level.
- **CLI:** All CLI operations log success at INFO and failure at WARN using shared helpers in `internal/logger/cli.go`.
- **Fields:** Use stable field names: `method`, `path`, `status_code`, `duration`, `request_id`, `user`, `username`, `operation`, `source` ("CLI" or "API"), and `error`.

## Reverse Engineering
Since the goal of this project is to replace an existing server application and talk to an existing client application, what we know about the content of their communications is listed below.

### KOReader Client Request Types and Payloads
These request headers and content are confirmed.

1.  Register
    - `POST` 'SERVER_DOMAIN/users/create'
    - `-H` 'host: SERVER_DOMAIN'
    - `-H` 'te: trailers'
    - `-H` 'content-type: application/json'
    - `-H` 'user-agent: lua-Spore'
    - `-H` 'connection: TE'
    - `-H` 'content-length: 67'
    - `-H` 'accept: application/vnd.koreader.v1+json'
    - `-d` $'{"password":"5f4dcc3b5aa765d61d8327deb882cf99","username":"USERNAME"}'
2.  Login
    - `GET` 'SERVER_DOMAIN/users/auth'
    - `-H` 'host: SERVER_DOMAIN'
    - `-H` 'te: trailers'
    - `-H` 'x-auth-user: USERNAME'
    - `-H` 'user-agent: lua-Spore'
    - `-H` 'connection: TE'
    - `-H` 'accept: application/vnd.koreader.v1+json'
    - `-H` 'x-auth-key: 5f4dcc3b5aa765d61d8327deb882cf99'
3.  Get Progress
    - `GET` 'SERVER_DOMAIN/syncs/progress/da58521cd09590a89fd378e5419e3987'
    - `-H` 'host: SERVER_DOMAIN'
    - `-H` 'te: trailers'
    - `-H` 'x-auth-user: USERNAME'
    - `-H` 'user-agent: lua-Spore'
    - `-H` 'connection: TE'
    - `-H` 'accept: application/vnd.koreader.v1+json'
    - `-H` 'x-auth-key: 5f4dcc3b5aa765d61d8327deb882cf99'
4.  Send Progress
    - `PUT` 'SERVER_DOMAIN/syncs/progress'
    - `-H` 'host: SERVER_DOMAIN'
    - `-H` 'te: trailers'
    - `-H` 'x-auth-user: USERNAME'
    - `-H` 'x-auth-key: 5f4dcc3b5aa765d61d8327deb882cf99'
    - `-H` 'content-type: application/json'
    - `-H` 'user-agent: lua-Spore'
    - `-H` 'connection: TE'
    - `-H` 'accept: application/vnd.koreader.v1+json'
    - `-H` 'content-length: 190'
    - `-d` $'{"percentage":0.6956,"document":"da58521cd09590a89fd378e5419e3987","device_id":"BCD651369D514D4B981C1B76CFDBAB5C","progress":"/body/DocFragment[14]/body/p[7]/text()[1].47","device":"caiman"}'

### KOReader Server Response Types and Payloads
These response headers and content are not confirmed, but it is suspected this is how the official KOReader Sync Server responds and what the KOReader client expects.

**Note:** Response codes (specifically 201 Created / 204 No Content for PUT) must be verified with a real client as soon as implementation allows.

| Endpoint                          | Success Code | Expected Response Body (JSON)                                                            |
| :-------------------------------- | :----------: | :--------------------------------------------------------------------------------------- |
| POST /users/create                | 201 Created  | {"username": "USERNAME", "message": "User created"}                                      |
| GET /users/auth                   | 200 OK       | {"authorized": "OK"}                                                                     |
| GET /syncs/progress/{DOCUMENT_ID} | 200 OK       | Probably the same progress JSON object sent by the client, and probably with a timestamp |
| PUT /syncs/progress               | 200 OK       | {"message": "Progress updated"}                                                          |


## Implementation Strategy

### Database Schema

- Use a pure Go SQLite: `modernc.org/sqlite`.
- Use WAL (Write-Ahead Logging) mode and `SetMaxOpenConns(1)` for optimal stability.
- Use two tables:
  - Users:
    | Column        | Type | Constraints |
    | :-----------: | :--: | :---------: |
    | username      | TEXT | PRIMARY KEY |
    | password_hash | TEXT | NOT NULL    |
  - Progress:
    | Column      | Type    | Constraints                |
    | :---------: | :-----: | :------------------------- |
    | username*   | TEXT    | REFERENCES Users(username) |
    | document*   | TEXT    | MD5 hash of the book       |
    | percentage  | REAL    |                            |
    | progress    | TEXT    | The XPath/CFI location     |
    | device_id   | TEXT    |                            |
    | device      | TEXT    |                            |
    | timestamp   | INTEGER | Unix epoch (Server-side)   |
    
    *Progress PRIMARY KEY (username, document)


### Server Architecture
Organize the code into three main layers: Middleware, Handlers, and Storage.

1. **Authentication Middleware**

Since nearly every request (except registration) requires the `X-AUTH-USER` and `X-AUTH-KEY` headers, you should write a middleware function. This function intercepts every request, checks the database for the user, and confirms the key matches. If it fails, return a `401 Unauthorized`.

2. **The Endpoints (Handlers)**

You’ll need to map your findings to Go functions:
    - `POST /users/create`: Decode the JSON body, check if the username is taken, and save to the DB.
    - `GET /users/auth`: Simply return `{"authorized": "OK"}` if the middleware passes.
    - `GET /syncs/progress/{DOCUMENT_ID}`: Get `document` (MD5) from the URL. Query the `Progress` table for that `document` and `username`. If found, return the JSON record. If not, return `404 Not Found`.
    - `PUT /syncs/progress`: Decode the JSON body. Upsert the record (Update if it exists for that user/book, Insert if it doesn’t).

### Structuring the Data

Define a struct that matches the JSON payload exactly.

```Go
type Progress struct {
    Document   string  `json:"document"`
    Percentage float64 `json:"percentage"`
    Progress   string  `json:"progress"`
    DeviceID   string  `json:"device_id"`
    Device     string  `json:"device"`
    Timestamp  int64   `json:"timestamp"` // Server-side arrival time
}
```

### The Workflow

- Initialize: Setup the SQLite connection and create tables if they don't exist.
- Routing: Use `net/http` (standard library).  But this means JSON will need to be manually encoded and decoded, and auth/logging will need manual wrapping.
- Compatibility: KOReader is notoriously picky about the `Accept: application/vnd.koreader.v1+json` header. In `net/http`, you can easily check this header manually to ensure strict compatibility:
- The "Sync" Logic: When a `PUT` comes in, don't just overwrite the data. Check the `timestamp`. If the data on the server is newer than the data being sent, ignore the update.

### Pitfalls to Watch Out For

- The Content-Type Header: KOReader is picky. Ensure your server always sends `Content-Type: application/vnd.koreader.v1+json`. If you send `application/json`, the client might ignore the response.
- URL Encoding: Sometimes the document IDs can contain characters that need careful handling in the URL path.

### Security

1. MD5 Logic: Remember that KOReader sends the MD5 of the password. Do not store this MD5 as-is. Salt and hash it again on your server (using `Bcrypt`) before saving it to your database for real security.
2. SQL Injection: Ensure that a malicious payload in the `document_id` or `username` can't wipe the database.
  - Never use `fmt.Sprintf` to build SQL queries.
  - Always use parameterized queries (placeholders).
3. Rate Limiting: The app must protect itself from brute-force login attempts or a "runaway" client that syncs every second.
  - Application Level: Use a package like `golang.org/x/time/rate`.
  - Proxy Level: Add advice to the README that if using Caddy, the user can add the `rate_limit` directive to drop requests from IPs that are hitting your `/users/auth` or `/users/create` endpoints too fast.
4. Defense in Depth: Add advice to the README to cover the following items.
  - Run as Non-Root: Create a dedicated system user just for your Go binary (e.g., a user named `kosync`).
  - Firewall (UFW): Close all ports except 80 and 443 (for Caddy). The Go app's internal port (e.g., 8081) should not be accessible from the outside world, only from `localhost` (where Caddy is).
  - Fail2Ban: Install `fail2ban` to automatically block IP addresses that repeatedly try to guess your SSH password or trigger 401 errors on your sync server.
5. Secure Error Handling: Be careful with what your server tells the world when things go wrong.
  - Vague is Better: If a login fails, return `401 Unauthorized`. Don't return `"Error: User 'bob' not found"` or `"Error: Wrong password for 'bob'"`. This prevents "User Enumeration," where an attacker figures out which usernames exist.
  - No Stack Traces: Ensure that in a production environment, your Go app doesn't print internal database errors or line numbers to the HTTP response. Log those internally, but give the user a generic `500 Internal Server Error`.
6. Database Backups: Security also means Availability. If your SQLite file gets corrupted or you accidentally delete it, your reading progress is gone.
  - Simple Strategy: Since SQLite is just a file, a daily cron job that copies the `.db` file to a secure cloud location (or another machine) is usually enough for a personal server. Just ensure you use the `.backup` command in the SQLite CLI to avoid copying the file while a "write" is in progress.  Add a sample code snippet in the README to help users implement this.

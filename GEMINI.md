# KOSYNC - Lightweight KOReader Position Sync Server

KOSYNC is a lightweight server designed to facilitate synchronization of ebooks across all of a user's KOReader devices.  KOReader essentially treats the server as a remote key-value store, so the main task is building a simple CRUD (Create, Read, Update, Delete) API.  There is an official KOReader Sync Server that ships with KOReader, but it has a number of drawbacks.  The intended audience for KOSYNC is home labbers, who may be trying to self-host on very resource constrained hardware.  The code and the README needs to be very well documented to assist novices understand how it works and how to deploy  and troubleshoot it.

## Project Overview

- **Core Technologies:**
  - **Language:** Go (Golang) for a single-binary, low-memory footprint. Version: 1.22.
  - **Database:** Pure Go SQLite (`modernc.org/sqlite`) for local indexing and multi-user support.
  - **Web Framework:** Lightweight routing with Go's standard HTTP router (`net/http.ServeMux`).
  - **Security:** Bcrypt (`golang.org/x/crypto/bcrypt`) for password hashing.
- **Architecture:**
  - **Clean Architecture:** Separation of domain logic, databases, UI, frameworks. (Note: For simplicity and novice-friendliness, consider keeping database logic in a single file within the storage layer).
  - **Deployment:** Ships as a standalone, single-executable binary for bare-metal execution, or in a Docker container.

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


## Roadmap
At the beginning of work on each step, prior to making any changes to any code, place a mark in the box next to that step (eg. [-]) so that it is clear what work was being done if there is a sudden interruption. After the completion of each step, git commit the changes with a descriptive message, then update this document to show the current state of progress by checking the box next to the step (eg. [x]).

### Phase 1: Environment & Scaffolding
Set up the Go workspace using the standard library to minimize dependencies and ensure long-term maintainability.

- [x] **1.1 Initialize Module:** Run `go mod init kosync`.
- [x] **1.2 Install Core Dependencies:** 
  - `go get modernc.org/sqlite` (Pure Go SQLite driver).
  - `go get golang.org/x/crypto/bcrypt` (Secure password hashing).
  - `go get golang.org/x/time/rate` (Rate limiting).
- [x] **1.3 Define Configuration:** Use Environment Variables:
  - `KOSYNC_PORT` (default 8081).
  - `KOSYNC_DB_PATH` (`kosync.db`).
  - `KOSYNC_LOG_LEVEL`.
  - `KOSYNC_DISABLE_REGISTRATION` (bool).
  - `KOSYNC_STORAGE_CAP_MB` (optional).
- [x] **1.4 Logging:** Implement structured logging using `log/slog`.
- [x] **1.5 Define Data Structures:** Create structs with JSON tags matching the KOReader protocol:
  - `User`: `username` and `password_hash`.
  - `Progress`: `document`, `percentage`, `progress`, `device_id`, `device`, and `timestamp`.

-----

### Phase 2: The Data Layer & Password Security
Implement the persistence logic and protect user credentials.

- [x] **2.1 Initialize DB:** Write `initDB()` to open SQLite and create tables:
  - `Users`: `username` (PK), `password_hash`.
  - `Progress`: `username`, `document`, `percentage`, `progress`, `device_id`, `device`, `timestamp`. Use a **Composite Primary Key** on `(username, document)`.
  - **Security:** Ensure the database file is created with `0600` permissions. Enable WAL mode and set `SetMaxOpenConns(1)`.
- [x] **2.2 Password Security:** Implement `HashPassword` and `CheckPassword` functions using **Bcrypt** (Cost factor 12) to secure the `X-AUTH-KEY` received from the client.
- [x] **2.3 Progress Logic:**
  - Write `GetProgressDB(user, docID)`: Returns the most recent record.
  - Write `UpsertProgressDB(Progress)`: Uses `INSERT ... ON CONFLICT(...) DO UPDATE` with prepared statements.
- [x] **2.4 CLI User Management:** Implement commands/functions for:
  - Creating a user.
  - Deleting a user.
  - Changing a user's password.
- [x] **2.5 Storage Management:** Implement logic to monitor database size and cull oldest records if `KOSYNC_STORAGE_CAP_MB` is exceeded.

-----

### Phase 3: Middleware & Authentication
Handle the "plumbing" of the KOReader protocol, specifically the custom headers.

- [x] **3.1 Auth Middleware:** Create a wrapper that validates `X-AUTH-USER` and `X-AUTH-KEY` against the database for every protected request.
- [x] **3.2 Header Validation:** Create middleware to ensure the `Accept` header matches `application/vnd.koreader.v1+json`.
- [x] **3.3 Content-Type Enforcement:** Ensure the server always responds with the correct KOReader-specific MIME type.
- [x] **3.4 Application Rate Limiting:** Implement rate limiting per IP for registration and auth endpoints.

-----

### Phase 4: API Handlers
Map the four specific requests identified during reverse engineering.  Handle any errors gracefully.

- [x] **4.1 User Registration (`POST /users/create`):** Decode JSON, check `KOSYNC_DISABLE_REGISTRATION`, bcrypt the password hash, and save. Handle existing users with a random delay.
- [x] **4.2 Auth Check (`GET /users/auth`):** Return `200 OK` with `{"authorized": "OK"}` if the middleware passes.
- [x] **4.3 Get Progress (`GET /syncs/progress/{document}`):** Sanitize/validate `document` (MD5 format), query DB, and return JSON.
- [x] **4.4 Update Progress (`PUT /syncs/progress`):** Decode the JSON body, sanitize inputs, and call the Upsert function. Return `201 Created` or `204 No Content` (Verify with real client).

-----

### Phase 5: Server Plumbing
Stitch everything into the main entry point using Go 1.22's enhanced `net/http` router.

- [x] **5.1 Route Registration:** 
  - `mux.HandleFunc("POST /users/create", handleUserCreate)`
  - `mux.HandleFunc("GET /users/auth", handleAuth)`
  - `mux.HandleFunc("GET /syncs/progress/{document}", handleGetProgress)`
  - `mux.HandleFunc("PUT /syncs/progress", handleUpdateProgress)`
- [x] **5.2 Graceful Shutdown:** Implement `os/signal` handling to ensure the SQLite database closes correctly when the process is stopped.

-----

### Phase 6: Testing & Validation
- [x] **6.1 Unit Tests:** Create tests for every function and wrapper.  Test for correct responses with both successful requests and unsuccessful requests.
    - Test the auth middleware to ensure the wrapper validates `X-AUTH-USER` and `X-AUTH-KEY` against the database, and that invalid credentials fail.
    - Test that the header validation ensures the `Accept` header always matches `application/vnd.koreader.v1+json`.
    - Test that the server always responds with the correct KOReader-specific MIME type.
    - Test that user registration correctly decodes the JSON object, bcrypt creates the password hash, and it is saved to the `Users` table.  Test that new user creation and attempts to register with an existing username are handled as per step 4.1.
    - Test that the auth check returns `200 OK` for valid users and `401 Unathorized` for invalid users.
    - Test that successful and unsuccessful requests for progress are handled per step 4.3.
    - Test the "Upsert" logic to ensure newer timestamps correctly overwrite older data.
    - Test the DB logic.  Write a test that upserts progress to a test database then retrieves progress to ensure the data persists.
    - Test that `Ctl-C` or other server shutdowns lead to SQLite closing correctly.
- [x] **6.2 Integration Test:** Write tests that use `curl` to simulate the full flow: Register -> Auth -> Put Progress -> Get Progress.
- [-] **6.3 Device Verification:** Perform a sync on an actual e-reader and verify the SQLite file updates.

-----

### Phase 7: Deployment Documentation (README Tasks)
Create a `README.md` focused on helping novices deploy the server safely.

- [x] **7.1 Reverse Proxy Guide:** Write a section on configuring **Caddy** to handle HTTPS/SSL.
- [x] **7.2 Security Best Practices:** 
  - Add instructions for running the Go binary as a non-root user.
  - Add instructions for setting up a basic firewall (UFW).
- [x] **7.3 SQLite Maintenance:** Explain how to back up the `.db` file using the SQLite `.backup` command.
- [x] **7.4 Novice-Friendly Setup:** Provide a sample `docker-compose.yaml` that bundles `kosync` and `Caddy` together for one-click deployment.

-----

### Phase 8: Unfinished Business
Missing or incorrectly implemented features from the earlier phases.

- [ ] **8.1 CLI User Management:** The desired outcome of step 2.5 was a command line UI for the server admin to use to create and delete users, as well as change user passwords.  Instead, functions were written in `storage.go` to talk to the database about usernames and password, but there was no UI implemented.  In subsequent edits an attempt was made to implement a user management command line UI in `main.go`, but this also does not meet expectations.
  - [x] **8.1.1 Change Password:** Add a `change-password` command.
  - [x] **8.1.2 CLI Format:** Refactor the `create-user`, `delete-user`, and `change-password` commands to take the format `./kosync COMMAND_NAME USERNAME` so there is no need to pass a `-u` flag every time for username.
  - [x] **8.1.3 Refactor Password Input Techniques:** To avoid having the passwords show up in the shell history, implement a hidden, interactive password input emulating the techniques used in KOPDS.  And like in KOPDS, there should be an option to pass an optional flag `--password-stdin` to re-enable non-interactive password entry for automation (for example `echo "mypassword" | docker exec -i kosync ./kosync create-user admin --password-stdin`). Use of `golang.org/x/term` is explicitly allowed to achieve this. Avoid requiring a version of Go later than 1.22.  See below for the code snipper from KOPDS:
  - [x] **8.1.4 Test Suite for User Management UI:**  Build tests for the user management UI.
- [x] **8.2 Project Documentation:** The goal of KOSYNC is to provide a server for home labbers of all experience levels.  The README.md documentation needs to reflect this.  Novices need the basics explained patiently and fully with the exact commands they'll need spelled out for them.  Experts need details about the implementation for debugging and contributing to the project.
  - [x] **8.2.1 CLI Usage Guide:** The README.md already contained installation instructions and KOReader usage instructions, but still needs instructions for using the newly build/refactored user management UI.
  - [x] **8.2.2 Log Level Explanation:** The log level options need a full explanation.
  - [x] **8.2.3 Expand Technical Overview:** The Technical Overview section of the README.md only mentions highlights.  Expand this to give a fuller explanation of how the app was built to work.
  - [x] **8.2.4 Expand Troublshooting:** The Troubleshooting section is also very light on detail.  Anticipate common usage and deployment issues and provide solutions.
- [x] **8.3 Database Consistency & Path Resolution:** Ensure the server and CLI always use the same database and prevent accidental file creation.
  - [x] **8.3.1 Absolute Path Resolution:** Modify `config.go` to resolve the database path to an absolute path relative to the executable by default.
  - [x] **8.3.2 Creation Safeguard:** Update `InitDB` to prevent the CLI from creating new database files.
  - [x] **8.3.3 CLI Path Logging:** Update the CLI to explicitly log the database path it is using for transparency.
- [x] **8.4 Logging Inconsistency:** When a user is created via an HTTP registration there is an event printed to the log. There is no entry in the log for any CLI user management commands, even when they fail.
  - [x] **8.4.1 Global Logger Initialization:** Move `InitLogger` to the top of `main()` in `main.go` so it is available to CLI commands.
  - [x] **8.4.2 CLI Logging:** Add `slog` calls to `runCLI` for success/failure of `create-user`, `delete-user`, and `change-password`, including a `source: CLI` attribute.
  - [x] **8.4.3 API Logging:** Update `handleUserCreate` in `handlers.go` to include a `source: API` attribute in its log entries.
  - [x] **8.4.4 Verification:** Verify that logs from both sources (CLI and API) correctly identify the source and username in the structured output.
  - [x] **8.4.5 Shared Log File Support:** Implement `KOSYNC_LOG_PATH` to allow unified logging to a file across separate processes (Server and CLI).



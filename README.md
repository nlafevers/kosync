# KOSYNC - Lightweight KOReader Sync Server

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/nlafevers/kosync)](https://goreportcard.com/report/github.com/nlafevers/kosync)

KOSYNC is a server that facilitates synchronization of ebooks across your KOReader devices. It is a lightweight, simple, and secure alternative to the official KOReader sync server, designed for self-hosting on resource-constrained hardware.

---

## 📖 Table of Contents

1.  [Why KOSYNC?](#-why-kosync)
2.  [Key Features](#-key-features)
3.  [Prerequisites](#-prerequisites)
4.  [Quick Start (Docker)](#-quick-start-docker)
5.  [Usage with KOReader](#-usage-with-koreader)
6.  [Native Installation](#-native-installation)
7.  [Configuration Reference](#-configuration-reference)
8.  [CLI User Management](#-cli-user-management)
9.  [Logging](#-logging)
10. [Technical Overview](#-technical-overview)
11. [Security](#-security)
12. [Troubleshooting](#-troubleshooting)
13. [License](#-license)

---

## 🚀 Why KOSYNC?

While official and alternative synchronization solutions exist, KOSYNC focuses on three core pillars:

1. **High Reliability:** By utilizing SQLite with Write-Ahead Logging (WAL) and strict ACID compliance, KOSYNC ensures your reading progress is never lost or corrupted, even if your server experiences an unexpected power loss.
2. **Resource Efficiency:** Built in pure Go, KOSYNC is a featherweight, single-binary application with a minimal memory footprint. It is designed to be "set and forget," operating perfectly on everything from enterprise servers to the most resource-constrained home lab hardware (like a Raspberry Pi Zero).
3. **Privacy-First & Secure:** KOSYNC is designed for full self-hosting. Your reading habits and sync data never leave your infrastructure. With bcrypt-hashed credentials and hardened API endpoints, it ensures your data remains yours alone.

---

## ✨ Key Features

- **KOReader Protocol Compliance:** Fully compatible with the custom `application/vnd.koreader.v1+json` protocol, ensuring a native-feeling experience on your e-reader.
- **Security-First API:** Implements rate-limiting, hardened header validation, and user enumeration mitigation to protect against brute-force and probing attacks.
- **Zero-Maintenance Storage:** Uses a single, lightweight SQLite file for all user and progress data, making daily backups and data migrations a simple file-copy operation.
- **Developer-Friendly Architecture:** Clean, modular design (Middleware -> Handlers -> Storage) that makes it easy to audit, troubleshoot, or extend.
- **Production-Ready:** Includes structured `slog` logging, graceful shutdown handling, and support for both native binary and Dockerized deployments.

---

## 📋 Prerequisites

### Software Requirements

#### 1. If using Docker (Recommended)
You need Docker and Docker Compose installed. To check if you have them, run:
```bash
docker --version
docker compose version
```
*If you don't have them, follow the [official Docker installation guide](https://docs.docker.com/get-docker/).*

#### 2. If installing Natively
You need the Go compiler (v1.25.x or later). To check your version, run:
```bash
go version
```
*If you don't have it, download it from [go.dev](https://go.dev/dl/). No C compiler is required as KOSYNC uses a pure-Go SQLite driver.*

#### 3. Reverse Proxy (recommended for production)
KOSYNC uses HTTP Basic Authentication, which transmits credentials in plain text. For any internet-facing deployment you should place it behind an HTTPS reverse proxy (Caddy is a good pure-Go choice). Reverse-proxy setup instructions live in the KOSERVER project deployment guide.
> [!NOTE]
> A reverse proxy alone does not make your server completely secure.  You are responsible for properly configuring your server to meet your security needs.

### Hardware Requirements

One reason to prefer deploying natively with a Go binary is to minimize resource usage in constrained server setups.  A free-tier GCP e2-micro VM only has 1 GB of memory, and early Raspberry Pi's have even less.  Even if the overhead consumed by Docker is as low as often claimed 100-200 MB (and not closer 300-400 MB), that is still a significant proportion of your available RAM on a micro cloud VM or early-generation Raspberry Pi.  The Go binary running natively should consume only a tenth of that (10-20 MB).  Running your entire stack natively, if using Caddy (20-30 MB), would consume less than half the RAM of the Docker overhead by itself.

The other hardware requirements are potato-tier.  See recommended below:

| Specification | Native Dual (kopds + kosync) | Docker Dual (kopds + kosync) | Native kosync | Native kopds |
| :-----------: | :--------------------------: | :--------------------------: | :----------:  | :----------: |
| CPU           | 1 Core (1.0 GHz)             | 1 Core (1.0 GHz)    | 1 Core (Any speed) | 1 Core (1.0 GHz) |
| RAM (Idle)    | ~100 MB                      | ~350 MB                      | < 15 MB       | ~90 MB       |
| RAM (Minimum) | 512 MB*                      | 1 GB*<sup>†</sup>            | 64 MB         | 512 MB*      |
| Storage Space | ~250 MB                      | ~1.5 GB                      | ~25 MB        | ~200 MB      |
| Network       | 1+ Mbps                      | 1+ Mbps                      | < 1 Mbps      | 1+ Mbps      |

_*Assumes rclone is used to mount remote storage. A swap file is highly recommended to prevent Out-of-Memory (OOM) crashes during initial directory scans._

_†1 GB will likely not be sufficient if you intend to build your own Docker image locally_

---

## 🐳 Quick Start (Docker)

The easiest way to run KOSYNC is via Docker. This method ensures all dependencies are handled and simplifies updates.

### 1. Prepare Your Environment
Create a directory for KOSYNC and move into it:
```bash
mkdir ~/kosync && cd ~/kosync
```

### 2. Create Docker Compose File
Create a file named `deploy/docker-compose.yml` and paste the following content.

```yaml
services:
  kosync:
    image: ghcr.io/nlafevers/kosync:latest # or build: .
    container_name: kosync
    restart: unless-stopped
    ports:
      - "8081:8081"
    # Security hardening
    read_only: true
    tmpfs:
      - /tmp
    volumes:
      # Persistent storage for the SQLite database
      - kosync_data:/app/data
    environment:
      - KOSYNC_PORT=8081
      - KOSYNC_DATABASE_PATH=/app/data/kosync.db
      - KOSYNC_LOG_LEVEL=info
      - KOSYNC_JSON_LOG=true
      - KOSYNC_DISABLE_REGISTRATION=true
      - KOSYNC_STORAGE_CAP_MB=0

volumes:
  kosync_data:
```

### 3. Launch KOSYNC
Start the server in the background:
```bash
docker compose up -d
```

### 4. Create Your Admin User
KOSYNC requires authentication. Create your first user with the following command:
```bash
docker exec -it kosync ./kosync create-user admin
```
Follow the prompts to set a secure password.

> [!TIP]
> For automation, you can use the `--password-stdin` flag:
> `echo "mypassword" | docker exec -i kosync ./kosync create-user admin --password-stdin`

---

## 📱 Usage with KOReader

1.  Open **KOReader**.
2.  Tap the top menu (while viewing a book) and select the **Tools** icon (crossed wrench and screwdriver).
3.  Select **Progress sync** -> **Custom sync server**.
4.  Enter the URL: `http://your-server-ip:8081`
5.  Click `OK`.
6.  Select **Register/Login**
7.  Enter the **Username** and **Password** you created in Quick Start - Step 4.
8.  Click `Login`.
9.  Adjust sync settings according to personal preference.
10. Select **Push progress from this device now** to confirm sync is working.

---

## 🛠 Native Installation

For users who prefer running KOSYNC without Docker, you can use one of the provided binaries (see Releases), or build one yourself.

### 1. Build from Source
```bash
git clone https://github.com/nlafevers/kosync.git
```
or, to download only the latest branch without the entire commit history
```bash
git clone --depth 1 --branch $(curl -s https://api.github.com/repos/nlafevers/kosync/releases/latest | grep "tag_name" | cut -d '"' -f 4) https://github.com/nlafevers/kosync.git
```
then
```bash
cd kosync
go build -o kosync ./cmd/kosync
```

### 2. Run as a non-root user
Create a dedicated system user to run the service securely, and give it ownership of the binary and the data directory.
```bash
sudo useradd -r -s /usr/sbin/nologin kosync
sudo mkdir -p data
sudo chown -R kosync:kosync kosync data
```

### 3. Run the server
KOSYNC reads its settings from environment variables or a `config.yaml` file (see [Configuration Reference](#-configuration-reference) for every option). The defaults are sensible for a first run:
```bash
sudo -u kosync ./kosync create-user admin
sudo -u kosync ./kosync
```

---

## ⚙️ Configuration Reference

All settings can be provided as environment variables (prefixed with `KOSYNC_`) or in a `config.yaml` file placed in the working directory (or a `./config` subdirectory).

| Variable | Description | Default |
| :--- | :--- | :--- |
| `KOSYNC_PORT` | The port the server listens on. | `8081` |
| `KOSYNC_DATABASE_PATH` | Path where the database file will be stored. `KOSYNC_DB_PATH` remains supported as a legacy alias. | `./data/kosync.db` |
| `KOSYNC_LOG_LEVEL` | Logging verbosity (`debug`, `info`, `warn`, `error`). | `info` |
| `KOSYNC_JSON_LOG` | Enable structured JSON logging (best for Docker and log aggregators). | `false` |
| `KOSYNC_LOG_PATH` | Optional log file. When set, the server writes logs to this file **and** stderr; CLI commands log to this file only. | - |
| `KOSYNC_DISABLE_REGISTRATION` | Disable user registration endpoints. Set to `false` to allow new users to self-register. | `true` |
| `KOSYNC_STORAGE_CAP_MB` | Maximum database size in MB (0 to disable). | `0` |
| `KOSYNC_RATE_LIMIT_ENABLED` | Enable rate limiting on API requests. | `true` |
| `KOSYNC_RATE_LIMIT_PER_MINUTE` | Maximum requests allowed per minute per IP. | `30` |
| `KOSYNC_RATE_LIMIT_BURST` | Maximum burst size for rate limiting. | `10` |
| `KOSYNC_TRUST_PROXY_HEADERS` | Trust `X-Forwarded-For` headers for client IP detection (enable only behind a trusted reverse proxy). | `false` |

---

## 🖥 CLI User Management

KOSYNC includes a built-in CLI for managing users securely without exposing passwords in your shell history.

### Create a User
```bash
./kosync create-user <username>
```
You will be prompted to enter and confirm a password. The characters will not be visible. The command will fail if the user already exists. To change an existing user's password, use the `change-password` command.

### Change a Password
```bash
./kosync change-password <username>
```
Useful for resetting a user's password or regular security updates.

### Delete a User
```bash
./kosync delete-user <username>
```
This will permanently remove the user and all their reading progress from the database.

### Automated Setup (Non-interactive)
For Docker initialization or scripts, you can use the `--password-stdin` flag:
```bash
echo "mypassword" | ./kosync create-user admin --password-stdin
```

User-management commands create and migrate the configured database automatically, so initial setup can create the first user before the server has been started.

---

## 📊 Logging

KOSYNC uses structured `slog` logging to provide an audit trail of system events, user management actions, and reading progress synchronization. All API logs include a `request_id` to correlate multiple events (like auth success followed by a progress update) within a single request.

### Log Formats
- **Human-Readable (Default):** Optimized for terminal viewing.
- **JSON:** Structured output for log aggregators (e.g., Loki, ELK). Enable with `KOSYNC_JSON_LOG=true`.

### Log Destinations
When `KOSYNC_LOG_PATH` is set, the **server** writes structured logs to both stderr and that file. **CLI** commands (`create-user`, `delete-user`, `change-password`) write structured logs to the file only — or discard them when no path is set — so the terminal shows only the one-line human-readable result.

**Docker note:** `docker exec` runs in a separate process — its output goes directly to your terminal, not through Docker's logging driver. CLI user-management commands therefore never appear in `docker logs` regardless of log settings. If you need a persistent audit trail of CLI operations, set `KOSYNC_LOG_PATH` to a path on a mounted volume (e.g., `/app/data/kosync.log`) and read that file directly.

### Log Levels
- **`debug`:** Shows granular details including database interactions, auth success events, and timestamp-based sync resolution (e.g., why an update was skipped).
- **`info`:** Recommended for production. Shows server startup, CLI successes, and completed HTTP requests.
- **`warn`:** Logs handled issues like authentication failures (401), rate-limit hits, or storage cap pruning events.
- **`error`:** Logs critical failures like database write errors or server-side panics.

### Example Logs

**Progress Updated (INFO):**
`time=2026-05-27T10:00:00Z level=INFO msg="progress updated" method=PUT path="/syncs/progress" request_id=af7e5b5b username=alice document=doc123 percentage=0.5 source=API`

**Storage Cap Pruning (WARN):**
`time=2026-05-27T10:05:00Z level=WARN msg="storage cap exceeded" database_path="/data/kosync.db" current_size_mb=10.5 cap_mb=10.0`

**Sync Resolution (DEBUG):**
`time=2026-05-27T10:10:00Z level=DEBUG msg="upserting progress" username=alice document=doc123 percentage=0.5 timestamp=1779940888`

---

## 🏗 Technical Overview

KOSYNC is built with a focus on simplicity and extreme efficiency.

### Architecture
- **Layered Design:** Separates concerns into **Middleware** (Auth/Headers), **Handlers** (API Logic), and **Storage** (SQLite).
- **Go 1.25+ Standard Library:** Uses the enhanced `net/http` router for performant, dependency-free routing.
- **Strict Protocol Compliance:** Enforces the custom `application/vnd.koreader.v1+json` MIME type required by the KOReader client.

### Data Integrity & Performance
- **SQLite WAL Mode:** Enables concurrent reads and writes, preventing "Database is locked" errors while maintaining ACID compliance.
- **Atomic Operations:** Uses SQL transactions for user deletion to ensure associated reading progress is cleaned up reliably.
- **Sync Logic:** Implements a "last-writer-wins" strategy based on server-side timestamps to resolve sync conflicts between multiple devices.

### Security Implementation
- **Password Hashing:** Uses `bcrypt` (cost 12) to secure credentials. Note: KOReader sends an MD5 of the user's password; KOSYNC hashes this MD5 again with bcrypt before storage.
- **Path Resolution:** The server resolves its database path relative to the executable by default, ensuring consistency even when run from different directories.
- **CLI Database Setup:** User-management commands create and migrate the configured database automatically, matching KOPDS behavior.

---

## 🔒 Security

KOSYNC uses **HTTP Basic Authentication**. It is simple and widely compatible, but it transmits credentials in plain text, so you should **always run KOSYNC behind an HTTPS reverse proxy** (such as Caddy, Nginx, or Traefik).

Step-by-step deployment instructions — reverse proxy, firewall, backups, and running KOPDS and KOSYNC together — live in the KOSERVER project deployment guide.

> [!NOTE]
> A reverse proxy alone does not make your server completely secure.  You are responsible for properly configuring your server to meet your security needs.

---

## ❓ Troubleshooting

### Connection Issues
- **KOReader "Network Error":** Verify your server is reachable from your e-reader's IP. Ensure your firewall allows traffic on the configured port.
- **406 Not Acceptable:** KOReader is very picky about headers. Ensure you haven't modified the `AcceptMiddleware`.

### Configuration & Logs
If you encounter issues, the first step is to increase the log verbosity. Set `KOSYNC_LOG_LEVEL=debug` and restart the server. This will provide more context about request headers and database operations.

### Database Errors
- **Unexpected database path:** Check `KOSYNC_DATABASE_PATH`; `KOSYNC_DB_PATH` is still accepted as a legacy alias.
- **Permission Denied:** Ensure the user running the `kosync` binary has read/write permissions for both the `.db` file and the directory it resides in (for WAL temporary files).

---

## 📜 License

KOSYNC is released under the **GPL-3.0 License**. See the [LICENSE](LICENSE) file for details.

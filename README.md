# KOSYNC - Lightweight KOReader Sync Server

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/nlafevers/kosync)](https://goreportcard.com/report/github.com/nlafevers/kosync)

KOSYNC is a server that facilitates synchronization of ebooks across your KOReader devices. It is a lightweight, simple, and secure alternative to the official KOReader sync server, designed for self-hosting on resource-constrained hardware.

---

## 📖 Table of Contents

1.  [Why KOSYNC?](#-why-KOSYNC)
2.  [Key Features](#-key-features)
3.  [Prerequisites](#-prerequisites)
4.  [Quick Start (Docker)](#-quick-start-docker)
5.  [Usage with KOReader](#-usage-with-koreader)
6.  [Native Installation](#-native-installation)
7.  [Security & Deployment](#-security--deployment)
8.  [Technical Overview](#-technical-overview)
9.  [Troubleshooting](#-troubleshooting)
10. [License](#-license)

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
You need the Go compiler (v1.22+). To check your version, run:
```bash
go version
```
*If you don't have it, download it from [go.dev](https://go.dev/dl/). No C compiler is required as KOSYNC uses a pure-Go SQLite driver.*

#### 3. Reverse Proxy
While KOSYNC itself uses HTTP Basic Authentication, for security reasons you should place it behind a reverse proxy.  Caddy is recommended to keep a pure-Go environment.
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

*Assumes rclone is used to mount remote storage. A swap file is highly recommended to prevent Out-of-Memory (OOM) crashes during initial directory scans.

†1 GB will likely not be sufficient if you intend to build your own Docker image locally

---

## 🐳 Quick Start (Docker)

The easiest way to run KOSYNC is via Docker. This method ensures all dependencies are handled and simplifies updates.

### 1. Prepare Your Environment
Create a directory for KOSYNC and move into it:
```bash
mkdir ~/kosync && cd ~/kosync
```

### 2. Create Docker Compose File
Create a file named `docker-compose.yml` and paste the following content.

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
      - KOSYNC_DB_PATH=/app/data/kosync.db
      - KOSYNC_LOG_LEVEL=info
      - KOSYNC_DISABLE_REGISTRATION=false
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

### 2. Configure
KOSYNC can be configured via environment variables. 

```bash
# Set required environment variables
export KOSYNC_PORT=8081 \
       KOSYNC_DB_PATH=/app/data/kosync.db \
       KOSYNC_LOG_LEVEL=info \
       KOSYNC_DISABLE_REGISTRATION=false
```

### 3. Run as non-root user
Create a dedicated user to run the service securely.
```bash
sudo useradd -r -s /usr/sbin/nologin kosync
sudo chown kosync:kosync kosync kosync.db
```

### 4. Run the server
```bash
./kosync
```

---

## 🔒 Security & Deployment

### Reverse Proxy (HTTPS)
It is highly recommended to put KOSYNC behind a reverse proxy like **Caddy** for automatic HTTPS.
**Sample Caddyfile:**
```
your-domain.com {
    reverse_proxy localhost:8081
}
```

### Firewall (UFW)
Only expose ports 80/443 to the world.
```bash
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

### Backups
Since KOSYNC uses SQLite, simply backup the `.db` file daily using `sqlite3`:
```bash
sqlite3 kosync.db ".backup 'kosync_backup.db'"
```

---

## 🏗 Technical Overview
- **Architecture:** Clean layered architecture (Middleware -> Handlers -> Storage).
- **Communication:** Standard Go `net/http` router. Enforces `application/vnd.koreader.v1+json` MIME types.
- **Security:** Bcrypt (cost 12) for password hashing.
- **Database:** Pure Go SQLite (`modernc.org/sqlite`) with WAL mode enabled.

---

## ❓ Troubleshooting
- **Logs:** Run with `KOSYNC_LOG_LEVEL=debug` to see detailed request flows.
- **Support:** Open an issue if you encounter unexpected behavior.

---

## 📜 License

KOSYNC is released under the **GPL-3.0 License**. See the [LICENSE](LICENSE) file for details.

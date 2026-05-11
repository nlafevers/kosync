# KOSYNC - Lightweight KOReader Sync Server

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Go Report Card](https://goreportcard.com/badge/github.com/nlafevers/kopds)](https://goreportcard.com/report/github.com/nlafevers/kopds)

KOSYNC is a server that facilitates synchronization of ebooks across your KOReader devices. It is a lightweight, simple, and secure alternative to the official KOReader sync server, designed for self-hosting on resource-constrained hardware.

---

## 📖 Table of Contents

1.  [Why KOSYNC?](#-why-kosync)
2.  [Key Features](#-key-features)
3.  [Prerequisites](#-prerequisites)

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

### 1. Software Requirements

#### If using Docker (Recommended)
You need Docker and Docker Compose installed. To check if you have them, run:
```bash
docker --version
docker compose version
```
*If you don't have them, follow the [official Docker installation guide](https://docs.docker.com/get-docker/).*

#### If installing Natively
You need the Go compiler (v1.22+). To check your version, run:
```bash
go version
```
*If you don't have it, download it from [go.dev](https://go.dev/dl/). No C compiler is required as KOPDS uses a pure-Go SQLite driver.*

#### Reverse Proxy
While KOSYNC itself uses HTTP Basic Authentication, for security reasons you should place it behind a reverse proxy.  Caddy is recommended to keep a pure-Go environment.
> [!NOTE]
> A reverse proxy alone does not make your server completely secure.  You are responsible for properly configuring your server to meet your security needs.

### 2. Hardware Requirements

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

## Quick Start
KOSYNC is built in Go. It stores data in a single SQLite file, making backups and migrations trivial.



## Deployment Options

### Option 1: Binary Deployment (Recommended for Bare Metal)
1. **Download/Build:**
   ```bash
   git clone https://github.com/your-repo/kosync.git
   cd kosync
   go build -o kosync .
   ```
2. **Run as non-root user:**
   Create a dedicated user to run the service securely.
   ```bash
   sudo useradd -r -s /usr/sbin/nologin kosync
   sudo chown kosync:kosync kosync kosync.db
   ```
3. **Run the server:**
   ```bash
   ./kosync
   ```

### Option 2: Docker Deployment
1. **Create a `docker-compose.yaml`:**
   ```yaml
   services:
     kosync:
       image: your-repo/kosync:latest
       volumes:
         - ./data:/app/data
       ports:
         - "8081:8081"
       restart: unless-stopped
   ```
2. **Launch:** `docker-compose up -d`

## Security & Best Practices

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

## Technical Overview
- **Architecture:** Clean layered architecture (Middleware -> Handlers -> Storage).
- **Communication:** Standard Go `net/http` router. Enforces `application/vnd.koreader.v1+json` MIME types.
- **Security:** Bcrypt (cost 12) for password hashing.
- **Database:** Pure Go SQLite (`modernc.org/sqlite`) with WAL mode enabled.

## User Management
KOSYNC includes a built-in CLI for managing users directly from your terminal, bypassing the need for manual SQLite manipulation.

- **Create User:**
  ```bash
  ./kosync create-user -username myuser -password mypassword
  ```
- **Delete User:**
  ```bash
  ./kosync delete-user -username myuser
  ```


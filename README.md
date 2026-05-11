# KOSYNC: Lightweight KOReader Sync Server

KOSYNC is a lightweight, single-binary server designed to facilitate synchronization of ebooks across your KOReader devices. It acts as a remote key-value store, providing a simple, secure, and privacy-focused alternative to official sync servers, perfect for self-hosting on resource-constrained hardware.

## Quick Start
KOSYNC is built in Go. It stores data in a single SQLite file, making backups and migrations trivial.

### Prerequisites
- Go 1.22+ (for compiling from source)
- SQLite (built-in, no external dependency)

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

## Contributing & Troubleshooting
- **Logs:** Run with `KOSYNC_LOG_LEVEL=debug` to see detailed request flows.
- **Support:** Open an issue if you encounter unexpected behavior.

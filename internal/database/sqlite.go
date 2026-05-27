package database

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"kosync/internal/models"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db  *sql.DB
	log *slog.Logger
}

func OpenSQLite(path string, allowCreate bool) (*sql.DB, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		if !allowCreate {
			return nil, fmt.Errorf("database file does not exist: %s", path)
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to create db file with 0600: %w", err)
		}
		file.Close()
	} else if err == nil {
		if err := os.Chmod(path, 0600); err != nil {
			return nil, fmt.Errorf("failed to chmod 0600 on existing db file: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

func InitDB(path string, allowCreate bool) (*Storage, error) {
	db, err := OpenSQLite(path, allowCreate)
	if err != nil {
		return nil, err
	}

	if err := Migrate(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Storage{db: db, log: slog.Default()}, nil
}

func Migrate(db *sql.DB) error {
	usersTable := `
	CREATE TABLE IF NOT EXISTS users (
		username TEXT PRIMARY KEY,
		password_hash TEXT NOT NULL
	);`

	progressTable := `
	CREATE TABLE IF NOT EXISTS progress (
		username TEXT,
		document TEXT,
		percentage REAL,
		progress TEXT,
		device_id TEXT,
		device TEXT,
		timestamp INTEGER,
		PRIMARY KEY (username, document),
		FOREIGN KEY (username) REFERENCES users(username)
	);`

	if _, err := db.Exec(usersTable); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	if _, err := db.Exec(progressTable); err != nil {
		return fmt.Errorf("failed to create progress table: %w", err)
	}

	return nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) logger() *slog.Logger {
	if s != nil && s.log != nil {
		return s.log
	}
	return slog.Default()
}

// GetProgress retrieves the reading progress for a specific user and document.
func (s *Storage) GetProgress(username, document string) (*models.Progress, error) {
	log := s.logger()
	log.Debug("getting progress", "username", username, "document", document)

	query := `SELECT document, percentage, progress, device_id, device, timestamp FROM progress WHERE username = ? AND document = ?`
	row := s.db.QueryRow(query, username, document)

	var p models.Progress
	err := row.Scan(&p.Document, &p.Percentage, &p.Progress, &p.DeviceID, &p.Device, &p.Timestamp)
	if err == sql.ErrNoRows {
		log.Debug("progress not found", "username", username, "document", document)
		return nil, nil
	}
	if err != nil {
		log.Error("failed to get progress", "username", username, "document", document, "error", err)
		return nil, err
	}

	log.Debug("database progress retrieved", "operation", "get_progress", "username", username, "document", document, "percentage", p.Percentage)
	return &p, nil
}

// UpsertProgress inserts or updates the reading progress.
// It only updates if the incoming timestamp is newer than the existing one.
func (s *Storage) UpsertProgress(username string, p models.Progress) error {
	log := s.logger()
	log.Debug("upserting progress", "username", username, "document", p.Document, "percentage", p.Percentage, "timestamp", p.Timestamp)

	query := `
	INSERT INTO progress (username, document, percentage, progress, device_id, device, timestamp)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(username, document) DO UPDATE SET
		percentage = excluded.percentage,
		progress = excluded.progress,
		device_id = excluded.device_id,
		device = excluded.device,
		timestamp = excluded.timestamp
	WHERE excluded.timestamp > progress.timestamp;`

	_, err := s.db.Exec(query, username, p.Document, p.Percentage, p.Progress, p.DeviceID, p.Device, p.Timestamp)
	if err != nil {
		log.Error("failed to upsert progress", "username", username, "document", p.Document, "error", err)
		return err
	}

	log.Debug("progress upserted", "username", username, "document", p.Document, "percentage", p.Percentage)
	return nil
}

// CreateUser creates a new user with a password (which should be the MD5 hash from the client).
func (s *Storage) CreateUser(username, password string) error {
	return s.SaveUser(username, password)
}

// CreateUserIfNotExists creates a new user only if they don't already exist.
func (s *Storage) CreateUserIfNotExists(username, password string) error {
	log := s.logger()
	log.Debug("creating user if not exists", "username", username)

	var exists bool
	err := s.db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", username).Scan(&exists)
	if err != nil {
		log.Error("failed to check user existence", "username", username, "error", err)
		return err
	}
	if exists {
		log.Warn("user already exists", "username", username)
		return fmt.Errorf("user already exists")
	}

	_, err = s.db.Exec(`INSERT INTO users (username, password_hash) VALUES (?, ?)`, username, password)
	if err != nil {
		log.Error("failed to create user", "username", username, "error", err)
		return err
	}
	log.Debug("user created", "username", username)
	return nil
}

// SaveUser creates or updates a user with a password hash.
func (s *Storage) SaveUser(username, password string) error {
	log := s.logger()
	log.Debug("saving user", "username", username)

	_, err := s.db.Exec(`
		INSERT INTO users (username, password_hash) VALUES (?, ?)
		ON CONFLICT(username) DO UPDATE SET password_hash=excluded.password_hash`,
		username, password)
	if err != nil {
		log.Error("failed to save user", "username", username, "error", err)
		return err
	}
	return err
}

// GetUserHash retrieves the password hash for a user.
func (s *Storage) GetUserHash(username string) (string, error) {
	log := s.logger()
	log.Debug("getting user hash", "username", username)

	var hash string
	err := s.db.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&hash)
	if err != nil {
		log.Error("failed to get user hash", "username", username, "error", err)
		return "", err
	}
	return hash, nil
}

// UpdateUserPassword updates a user's password hash.
func (s *Storage) UpdateUserPassword(username, passwordHash string) error {
	log := s.logger()
	log.Debug("updating user password", "username", username)

	res, err := s.db.Exec("UPDATE users SET password_hash = ? WHERE username = ?", passwordHash, username)
	if err != nil {
		log.Error("failed to update user password", "username", username, "error", err)
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		log.Error("failed to count updated rows", "username", username, "error", err)
		return err
	}
	if rows == 0 {
		log.Warn("user not found for password update", "username", username)
		return fmt.Errorf("user not found")
	}
	return nil
}

// DeleteUser removes a user and their reading progress.
func (s *Storage) DeleteUser(username string) error {
	log := s.logger()
	log.Debug("deleting user", "username", username)

	tx, err := s.db.Begin()
	if err != nil {
		log.Error("failed to begin user deletion transaction", "username", username, "error", err)
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM progress WHERE username = ?", username); err != nil {
		log.Error("failed to delete user progress", "username", username, "error", err)
		return err
	}
	res, err := tx.Exec("DELETE FROM users WHERE username = ?", username)
	if err != nil {
		log.Error("failed to delete user", "username", username, "error", err)
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		log.Error("failed to count deleted rows", "username", username, "error", err)
		return err
	}
	if rows == 0 {
		log.Warn("user not found for deletion", "username", username)
		return fmt.Errorf("user not found")
	}

	if err := tx.Commit(); err != nil {
		log.Error("failed to commit user deletion", "username", username, "error", err)
		return err
	}
	return nil
}

// EnforceStorageCap checks if the database file exceeds the size limit.
// If it does, it deletes the oldest 20% of progress records and runs VACUUM.
func (s *Storage) EnforceStorageCap(path string, capMB int) (bool, error) {
	return enforceStorageCap(path, capMB, s.pruneStorageCapRecords, s.vacuum)
}

func (s *Storage) pruneStorageCapRecords() error {
	// Delete oldest 20% of progress records.
	_, err := s.db.Exec(`
		DELETE FROM progress
		WHERE (username, document) IN (
			SELECT username, document
			FROM progress
			ORDER BY timestamp ASC
			LIMIT (SELECT COUNT(*) / 5 FROM progress) + 1
		)`)
	return err
}

func (s *Storage) vacuum() error {
	_, err := s.db.Exec("VACUUM")
	return err
}

func enforceStorageCap(path string, capMB int, prune func() error, vacuum func() error) (bool, error) {
	if capMB <= 0 {
		return false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	if info.Size() < int64(capMB)*1024*1024 {
		return false, nil
	}

	if err := prune(); err != nil {
		return false, err
	}

	if err := vacuum(); err != nil {
		return false, err
	}

	return true, nil
}

package database

import (
	"database/sql"
	"fmt"
	"os"

	"kosync/internal/models"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db *sql.DB
}

func InitDB(path string, allowCreate bool) (*Storage, error) {
	// 2.1 Security: Ensure the database file is handled with 0600 permissions.
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
		// Ensure existing file has 0600
		if err := os.Chmod(path, 0600); err != nil {
			return nil, fmt.Errorf("failed to chmod 0600 on existing db file: %w", err)
		}
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	// 2.1 Security: Enable WAL mode and set SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable WAL: %w", err)
	}

	db.SetMaxOpenConns(1)

	s := &Storage{db: db}
	if err := s.createTables(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Storage) createTables() error {
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

	if _, err := s.db.Exec(usersTable); err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	if _, err := s.db.Exec(progressTable); err != nil {
		return fmt.Errorf("failed to create progress table: %w", err)
	}

	return nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

// GetProgress retrieves the reading progress for a specific user and document.
func (s *Storage) GetProgress(username, document string) (*models.Progress, error) {
	query := `SELECT document, percentage, progress, device_id, device, timestamp FROM progress WHERE username = ? AND document = ?`
	row := s.db.QueryRow(query, username, document)

	var p models.Progress
	err := row.Scan(&p.Document, &p.Percentage, &p.Progress, &p.DeviceID, &p.Device, &p.Timestamp)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpsertProgress inserts or updates the reading progress.
// It only updates if the incoming timestamp is newer than the existing one.
func (s *Storage) UpsertProgress(username string, p models.Progress) error {
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
	return err
}

// CreateUser creates a new user with a password (which should be the MD5 hash from the client).
func (s *Storage) CreateUser(username, password string) error {
	_, err := s.db.Exec("INSERT INTO users (username, password_hash) VALUES (?, ?)", username, password)
	return err
}

// GetUserHash retrieves the password hash for a user.
func (s *Storage) GetUserHash(username string) (string, error) {
	var hash string
	err := s.db.QueryRow("SELECT password_hash FROM users WHERE username = ?", username).Scan(&hash)
	return hash, err
}

// UpdateUserPassword updates a user's password hash.
func (s *Storage) UpdateUserPassword(username, passwordHash string) error {
	res, err := s.db.Exec("UPDATE users SET password_hash = ? WHERE username = ?", passwordHash, username)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

// DeleteUser removes a user and their reading progress.
func (s *Storage) DeleteUser(username string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM progress WHERE username = ?", username); err != nil {
		return err
	}
	res, err := tx.Exec("DELETE FROM users WHERE username = ?", username)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return tx.Commit()
}

// EnforceStorageCap checks if the database file exceeds the size limit.
// If it does, it deletes the oldest 20% of progress records and runs VACUUM.
func (s *Storage) EnforceStorageCap(path string, capMB int) (bool, error) {
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

	// Delete oldest 20% of progress records.
	_, err = s.db.Exec(`
		DELETE FROM progress 
		WHERE (username, document) IN (
			SELECT username, document 
			FROM progress 
			ORDER BY timestamp ASC 
			LIMIT (SELECT COUNT(*) / 5 FROM progress) + 1
		)`)
	if err != nil {
		return false, err
	}

	_, err = s.db.Exec("VACUUM")
	return true, err
}

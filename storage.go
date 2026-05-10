package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "modernc.org/sqlite"
)

type Storage struct {
	db *sql.DB
}

func InitDB(path string) (*Storage, error) {
	// 2.1 Security: Ensure the database file is created with 0600 permissions if it doesn't exist.
	if _, err := os.Stat(path); os.IsNotExist(err) {
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
